# Deployment Guide

plex-helper can be deployed via Docker (recommended) or as a systemd service.

## Prerequisites

1. Build the binary for Linux:
   ```bash
   GOOS=linux GOARCH=amd64 go build -o plex-helper .
   ```

2. Create your config file based on `config.example.json`

3. Configure the Plex webhook (required for instant detection):
   - Go to Plex Settings → Webhooks
   - Add webhook URL: `http://<your-server-ip>:8081/webhook`

## Option 1: Docker (Recommended)

### Dockerfile

```dockerfile
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY plex-helper /usr/local/bin/plex-helper
COPY config.json /etc/plex-helper/config.json
ENTRYPOINT ["plex-helper", "-config", "/etc/plex-helper/config.json"]
```

### Build and Run

```bash
# Build image
docker build -t plex-helper .

# Run container
docker run -d \
  --name plex-helper \
  --restart unless-stopped \
  --network host \
  plex-helper
```

Using `--network host` is simplest since plex-helper needs to reach:
- Plex on localhost
- qBittorrent on a remote server
- Expose port 8081 for webhooks

### docker-compose

```yaml
version: "3.8"
services:
  plex-helper:
    build: .
    container_name: plex-helper
    restart: unless-stopped
    network_mode: host
    volumes:
      - ./config.json:/etc/plex-helper/config.json:ro
```

Run with:
```bash
docker-compose up -d
```

### View Logs

```bash
docker logs -f plex-helper
```

## Option 2: Systemd Service

### Install Binary

```bash
sudo cp plex-helper /usr/local/bin/
sudo chmod +x /usr/local/bin/plex-helper
sudo mkdir -p /etc/plex-helper
sudo cp config.json /etc/plex-helper/config.json
```

### Create Service File

Create `/etc/systemd/system/plex-helper.service`:

```ini
[Unit]
Description=Plex Helper - Bandwidth Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/plex-helper -config /etc/plex-helper/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable plex-helper
sudo systemctl start plex-helper
```

### View Logs

```bash
journalctl -u plex-helper -f
```

## Verification

1. Check health endpoint:
   ```bash
   curl http://localhost:8081/health
   ```

2. Test webhook manually:
   ```bash
   curl -X POST http://localhost:8081/webhook \
     -F 'payload={"event":"media.play","Player":{"local":false}}'
   ```

3. Start a remote Plex stream and verify qBittorrent limit changes

## Updating

### Docker
```bash
# Rebuild with new binary
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

### Systemd
```bash
sudo systemctl stop plex-helper
sudo cp plex-helper /usr/local/bin/
sudo systemctl start plex-helper
```
