package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIHost  string `yaml:"api_host"`
	APIKey   string `yaml:"api_key"`
	Interval int    `yaml:"interval"`
	Buckets  struct {
		CPU     string `yaml:"cpu"`
		Memory  string `yaml:"memory"`
		Disk    string `yaml:"disk"`
		Network string `yaml:"network"`
	} `yaml:"buckets"`
}

type Fixed3 float64

func (f Fixed3) MarshalJSON() ([]byte, error) {
	v := float64(f)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatFloat(v, 'f', 3, 64)), nil
}

type Series struct {
	SK int    `json:"sk"`
	V  Fixed3 `json:"v"`
}

type Record struct {
	K string   `json:"k"`
	D []Series `json:"d"`
}

func series(sk int, v float64) Series {
	return Series{SK: sk, V: Fixed3(v)}
}

var client = &http.Client{Timeout: 10 * time.Second}

// loadConfig reads config.yaml from the same directory as the binary
func loadConfig() Config {
	exe, _ := os.Executable()
	path := os.Getenv("DASHGRID_CONFIG")
	if path == "" {
		path = filepath.Join(filepath.Dir(exe), "config.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("config parse: %v", err)
	}

	if cfg.APIHost == "" {
		log.Fatalf("config: api_host is required in %s", path)
	}

	if cfg.APIKey == "" {
		log.Fatalf("config: api_key is required in %s", path)
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 10
	}
	return cfg
}

// post sends records to a Dashgrid bucket
func post(cfg Config, bucket string, records []Record) {
	if bucket == "" {
		return
	}
	body, _ := json.Marshal(records)
	req, _ := http.NewRequest("POST", cfg.APIHost+"/api/buckets/"+bucket, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("  ERROR %s: %v", bucket, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var buf [512]byte
		n, _ := resp.Body.Read(buf[:])
		log.Printf("  ERROR %s: HTTP %d — %s", bucket, resp.StatusCode, string(buf[:n]))
	}
}

func readFile(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

// --- CPU ---

type cpuSnap struct{ user, nice, sys, idle, iowait, irq, sirq uint64 }

func (s cpuSnap) total() uint64 { return s.user + s.nice + s.sys + s.idle + s.iowait + s.irq + s.sirq }

func readCPU() cpuSnap {
	fields := strings.Fields(strings.SplitN(readFile("/proc/stat"), "\n", 2)[0])
	if len(fields) < 8 {
		return cpuSnap{}
	}
	p := func(s string) uint64 { v, _ := strconv.ParseUint(s, 10, 64); return v }
	return cpuSnap{p(fields[1]), p(fields[2]), p(fields[3]), p(fields[4]), p(fields[5]), p(fields[6]), p(fields[7])}
}

func cpuPct(prev, cur cpuSnap) float64 {
	dt := cur.total() - prev.total()
	if dt == 0 {
		return 0
	}
	return float64(dt-(cur.idle-prev.idle)) / float64(dt) * 100
}

func loadAvg() (float64, float64, float64) {
	f := strings.Fields(readFile("/proc/loadavg"))
	if len(f) < 3 {
		return 0, 0, 0
	}
	p := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	return p(f[0]), p(f[1]), p(f[2])
}

// --- Memory ---

func readMem() (total, used, avail int64) {
	var t, a int64
	for line := range strings.SplitSeq(readFile("/proc/meminfo"), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.ParseInt(f[1], 10, 64)
		switch f[0] {
		case "MemTotal:":
			t = v / 1024
		case "MemAvailable:":
			a = v / 1024
		}
	}
	return t, t - a, a
}

// --- Disk ---

func readDisk() (total, used, avail uint64) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err != nil {
		return 0, 0, 0
	}
	bs := uint64(fs.Bsize)
	total = fs.Blocks * bs / 1024 / 1024
	avail = fs.Bavail * bs / 1024 / 1024
	used = total - (fs.Bfree * bs / 1024 / 1024)
	return
}

// --- Network ---

type netSnap struct{ rx, tx uint64 }

func defaultIface() string {
	for _, line := range strings.Split(readFile("/proc/net/route"), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 2 && f[1] == "00000000" {
			return f[0]
		}
	}
	return ""
}

func readNet(iface string) netSnap {
	prefix := iface + ":"
	for line := range strings.SplitSeq(readFile("/proc/net/dev"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		fields := strings.Fields(strings.SplitN(line, ":", 2)[1])
		if len(fields) < 10 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		return netSnap{rx, tx}
	}
	return netSnap{}
}

func main() {
	cfg := loadConfig()
	iface := defaultIface()
	prevCPU := readCPU()
	prevNet := readNet(iface)

	log.Printf("started (interval: %ds, iface: %s)", cfg.Interval, iface)
	interval := time.Duration(cfg.Interval) * time.Second
	time.Sleep(interval)

	for {
		ts := time.Now().UTC().Format(time.RFC3339)
		r := func(d ...Series) []Record { return []Record{{K: ts, D: d}} }

		// CPU: usage%, load 1/5/15
		curCPU := readCPU()
		cpuPctVal := cpuPct(prevCPU, curCPU)
		l1, l5, l15 := loadAvg()
		post(cfg, cfg.Buckets.CPU, r(series(1, cpuPctVal), series(2, l1), series(3, l5), series(4, l15)))
		prevCPU = curCPU

		// Memory (MB): total, used, available
		mt, mu, ma := readMem()
		post(cfg, cfg.Buckets.Memory, r(series(1, float64(mt)), series(2, float64(mu)), series(3, float64(ma))))

		// Disk (MB): total, used, available
		dt, du, da := readDisk()
		post(cfg, cfg.Buckets.Disk, r(series(1, float64(dt)), series(2, float64(du)), series(3, float64(da))))

		// Network (KB/s): rx, tx
		curNet := readNet(iface)
		rxKB := float64(curNet.rx-prevNet.rx) / float64(cfg.Interval) / 1024
		txKB := float64(curNet.tx-prevNet.tx) / float64(cfg.Interval) / 1024
		prevNet = curNet
		post(cfg, cfg.Buckets.Network, r(series(1, rxKB), series(2, txKB)))

		log.Printf("[%s] cpu=%.1f%% mem=%d/%dMB disk=%d/%dMB net=%.2f/%.2f KB/s",
			ts, cpuPctVal, mu, mt, du, dt, rxKB, txKB)
		time.Sleep(interval)
	}
}
