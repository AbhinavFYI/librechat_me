# LibreChat Systemd Services

This directory contains systemd service files to run your LibreChat application 24/7 in production.

## Services Overview

| Service | Port | Description |
|---------|------|-------------|
| `frontend.service` | 3090 | LibreChat frontend (Node.js) |
| `librechat-backend.service` | 3080 | LibreChat backend (Node.js) |
| `go-api.service` | 8080 | Go backend API |
| `go-proxy.service` | 9443 | Go proxy service |

## Installation

### 1. Install Services

```bash
cd /home/ec2-user/librechat_me/systemd
chmod +x *.sh
./install-services.sh
```

This will:
- Copy service files to `/etc/systemd/system/`
- Enable services to start on boot
- Create logs directory

### 2. Start All Services

```bash
./start-all-services.sh
```

Or start individually:
```bash
sudo systemctl start frontend.service
sudo systemctl start librechat-backend.service
sudo systemctl start go-api.service
sudo systemctl start go-proxy.service
```

## Management Commands

### Check Status
```bash
./status-services.sh
```

Or check individual service:
```bash
sudo systemctl status frontend.service
```

### Stop All Services
```bash
./stop-all-services.sh
```

### Restart All Services
```bash
./restart-all-services.sh
```

### View Logs
```bash
# View specific service logs
./view-logs.sh frontend
./view-logs.sh librechat-backend
./view-logs.sh go-api
./view-logs.sh go-proxy

# View all logs
./view-logs.sh all

# Follow logs in real-time
sudo journalctl -u frontend.service -f
```

## Service Features

✅ **Auto-restart**: Services automatically restart if they crash  
✅ **Boot startup**: Services start automatically on system boot  
✅ **Logging**: All output logged to journalctl and log files  
✅ **Dependency management**: Services start in correct order  
✅ **Security**: Basic security hardening applied  

## Log Files

Logs are stored in two locations:

1. **Systemd journal** (recommended):
   ```bash
   sudo journalctl -u frontend.service
   ```

2. **Log files** (backup):
   - `/home/ec2-user/librechat_me/logs/frontend.log`
   - `/home/ec2-user/librechat_me/logs/librechat-backend.log`
   - `/home/ec2-user/librechat_me/logs/go-api.log`
   - `/home/ec2-user/librechat_me/logs/go-proxy.log`

## Troubleshooting

### Service won't start
```bash
# Check status and errors
sudo systemctl status frontend.service

# View detailed logs
sudo journalctl -u frontend.service -n 100

# Check if port is already in use
sudo netstat -tulpn | grep 3090
```

### Reload service after editing
```bash
sudo systemctl daemon-reload
sudo systemctl restart frontend.service
```

### Disable auto-start on boot
```bash
sudo systemctl disable frontend.service
```

### Re-enable auto-start on boot
```bash
sudo systemctl enable frontend.service
```

## Production Recommendations

For production use, consider:

1. **Build frontend for production** instead of using `npm run dev`:
   - Update `frontend.service` to use `npm run start` or serve built files with nginx
   
2. **Compile Go binaries** instead of using `go run`:
   - Build binaries: `go build -o api main.go`
   - Update service to run binary instead of `go run`

3. **Use environment files** for configuration:
   - Add `EnvironmentFile=/path/to/.env` in service files

4. **Set up log rotation** to prevent disk space issues

5. **Monitor services** with tools like Prometheus or Grafana

## Uninstall

To remove services:
```bash
sudo systemctl stop frontend.service librechat-backend.service go-api.service go-proxy.service
sudo systemctl disable frontend.service librechat-backend.service go-api.service go-proxy.service
sudo rm /etc/systemd/system/frontend.service
sudo rm /etc/systemd/system/librechat-backend.service
sudo rm /etc/systemd/system/go-api.service
sudo rm /etc/systemd/system/go-proxy.service
sudo systemctl daemon-reload
```

