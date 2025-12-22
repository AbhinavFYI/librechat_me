#!/bin/bash

# Script to restart all LibreChat services

echo "Restarting all services..."

sudo systemctl restart frontend.service
sudo systemctl restart librechat-backend.service
sudo systemctl restart go-api.service
sudo systemctl restart go-proxy.service

echo ""
echo "✅ All services restarted!"
echo ""
echo "Check status with:"
echo "  ./status-services.sh"

