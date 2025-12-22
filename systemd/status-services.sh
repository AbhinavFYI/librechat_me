#!/bin/bash

# Script to check status of all LibreChat services

echo "========================================"
echo "Frontend Service (Port 3090)"
echo "========================================"
sudo systemctl status frontend.service --no-pager -l

echo ""
echo "========================================"
echo "LibreChat Backend Service (Port 3080)"
echo "========================================"
sudo systemctl status librechat-backend.service --no-pager -l

echo ""
echo "========================================"
echo "Go API Service (Port 8080)"
echo "========================================"
sudo systemctl status go-api.service --no-pager -l

echo ""
echo "========================================"
echo "Go Proxy Service (Port 9443)"
echo "========================================"
sudo systemctl status go-proxy.service --no-pager -l

