#!/bin/bash

# Script to stop all LibreChat services

echo "Stopping all services..."

sudo systemctl stop frontend.service
sudo systemctl stop librechat-backend.service
sudo systemctl stop go-api.service
sudo systemctl stop go-proxy.service

echo ""
echo "✅ All services stopped!"

