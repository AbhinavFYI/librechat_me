#!/bin/bash

# Script to install and enable systemd services for LibreChat and Go backends

set -e

echo "Installing systemd services..."

# Create logs directory if it doesn't exist
mkdir -p /home/ec2-user/librechat_me/logs

# Copy service files to systemd directory
sudo cp /home/ec2-user/librechat_me/systemd/frontend.service /etc/systemd/system/
sudo cp /home/ec2-user/librechat_me/systemd/librechat-backend.service /etc/systemd/system/
sudo cp /home/ec2-user/librechat_me/systemd/go-api.service /etc/systemd/system/
sudo cp /home/ec2-user/librechat_me/systemd/go-proxy.service /etc/systemd/system/

# Reload systemd to recognize new services
sudo systemctl daemon-reload

# Enable services to start on boot
sudo systemctl enable frontend.service
sudo systemctl enable librechat-backend.service
sudo systemctl enable go-api.service
sudo systemctl enable go-proxy.service

echo ""
echo "✅ Services installed and enabled!"
echo ""
echo "To start all services, run:"
echo "  sudo systemctl start frontend.service"
echo "  sudo systemctl start librechat-backend.service"
echo "  sudo systemctl start go-api.service"
echo "  sudo systemctl start go-proxy.service"
echo ""
echo "Or use the start-all-services.sh script"

