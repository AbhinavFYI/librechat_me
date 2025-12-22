#!/bin/bash

# Script to start all LibreChat services

echo "Starting all services..."

sudo systemctl start frontend.service
sudo systemctl start librechat-backend.service
sudo systemctl start go-api.service
sudo systemctl start go-proxy.service

echo ""
echo "✅ All services started!"
echo ""
echo "Check status with:"
echo "  ./status-services.sh"

