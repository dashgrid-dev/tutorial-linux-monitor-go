# Dashgrid Linux Monitor (Go Version)

Single Go binary that pushes system metrics to [Dashgrid](https://dashgrid.com) via REST API. No runtime dependencies.

## Prerequisites

1. Create a Dashgrid account and get an API key
2. Create 4 TSV data buckets per host (CPU, Memory, Disk, Network)

## Metrics

| Bucket | Series 1 | Series 2 | Series 3 | Series 4 |
|--------|----------|----------|----------|----------|
| CPU | usage % | load 1m | load 5m | load 15m |
| Memory | total MB | used MB | available MB | — |
| Disk | total MB | used MB | available MB | — |
| Network | rx KB/s | tx KB/s | — | — |

### CPU
- **usage %** — percentage of CPU time spent working (not idle) since last sample. Computed as delta from `/proc/stat`.
- **load 1m / 5m / 15m** — average number of processes waiting to run over the last 1, 5, and 15 minutes. A load of 1.0 on a single-core machine means it's fully busy. Scale by number of cores.

### Memory
- **total MB** — total physical RAM installed.
- **used MB** — RAM actively in use (total minus available).
- **available MB** — RAM that can be allocated to processes without swapping. Includes free memory and reclaimable caches.

### Disk
- **total / used / available MB** — root filesystem (`/`) capacity. Uses `statfs` syscall, so values reflect the actual usable space (accounting for reserved blocks).

### Network
- **rx KB/s** — kilobytes per second received (download) on the default network interface.
- **tx KB/s** — kilobytes per second transmitted (upload). Both are computed as deltas between samples.

## Build

Cross-compile from macOS. Output goes into `src/build/` alongside `setup.sh` and `config.yaml`, so the whole directory can be `scp`'d to the host:

```bash
cd src/monitor
GOOS=linux GOARCH=amd64 go build -o ../build/dashgrid-monitor-linux-amd64 .
```

## Configure

Edit `src/build/config.yaml` with your API URL, API key, and bucket IDs:

```yaml
api_host: "your-api-host-here"
api_key: "your-api-key-here"
interval: 10

buckets:
  cpu: "bucket-id-cpu"
  memory: "bucket-id-memory"
  disk: "bucket-id-disk"
  network: "bucket-id-network"
```

## Deploy

Copy the build directory to each host and run the installer:

```bash
scp src/build/dashgrid-monitor-linux-amd64 src/build/setup.sh src/build/config.yaml user@vm:~/monitor/
ssh user@vm 'sudo bash ~/monitor/setup.sh'
```

## Verify

```bash
sudo journalctl -u dashgrid-monitor -f
```

## Update binary

Stop the service before copying (the running binary is locked):

```bash
ssh user@vm 'sudo systemctl stop dashgrid-monitor'
scp src/build/dashgrid-monitor-linux-amd64 user@vm:~/monitor/
ssh user@vm 'sudo systemctl start dashgrid-monitor'
```

## Restart

After editing `~/monitor/config.yaml` on the host:

```bash
sudo systemctl restart dashgrid-monitor
```

## Uninstall

```bash
sudo systemctl disable --now dashgrid-monitor
sudo rm /etc/systemd/system/dashgrid-monitor.service
sudo systemctl daemon-reload
```
