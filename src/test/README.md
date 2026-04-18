# Local testing (Docker)

Run the monitor in a Docker container to verify it pushes to Dashgrid Data Buckets — no real Linux host needed.

## Setup

```bash
cp config.test.yaml.example config.test.yaml
# edit config.test.yaml with your api_host, api_key, and bucket IDs
```

`config.test.yaml` is gitignored — your keys stay local.

## Run

```bash
bash test-docker.sh
```

The script cross-compiles `dashgrid-monitor-linux-amd64`, mounts this directory into a `debian:stable-slim` container, and runs the monitor against the `api_host` in `config.test.yaml`.

Ctrl-C to stop. First sample appears after one `interval` (baseline for CPU/network deltas).

Metrics reflect the *container*, not your Mac — CPU/memory are the container's share, disk is the Docker Desktop VM's virtual disk, network is the container's veth.
