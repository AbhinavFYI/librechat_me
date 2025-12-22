#!/bin/bash

# Script to view logs for LibreChat services

if [ -z "$1" ]; then
    echo "Usage: ./view-logs.sh [frontend|librechat-backend|go-api|go-proxy|all]"
    echo ""
    echo "Examples:"
    echo "  ./view-logs.sh frontend          # View frontend logs"
    echo "  ./view-logs.sh go-api            # View Go API logs"
    echo "  ./view-logs.sh all               # View all logs"
    exit 1
fi

case "$1" in
    frontend)
        echo "=== Frontend Logs ==="
        sudo journalctl -u frontend.service -n 50 --no-pager
        ;;
    librechat-backend)
        echo "=== LibreChat Backend Logs ==="
        sudo journalctl -u librechat-backend.service -n 50 --no-pager
        ;;
    go-api)
        echo "=== Go API Logs ==="
        sudo journalctl -u go-api.service -n 50 --no-pager
        ;;
    go-proxy)
        echo "=== Go Proxy Logs ==="
        sudo journalctl -u go-proxy.service -n 50 --no-pager
        ;;
    all)
        echo "=== Frontend Logs ==="
        sudo journalctl -u frontend.service -n 20 --no-pager
        echo ""
        echo "=== LibreChat Backend Logs ==="
        sudo journalctl -u librechat-backend.service -n 20 --no-pager
        echo ""
        echo "=== Go API Logs ==="
        sudo journalctl -u go-api.service -n 20 --no-pager
        echo ""
        echo "=== Go Proxy Logs ==="
        sudo journalctl -u go-proxy.service -n 20 --no-pager
        ;;
    *)
        echo "Invalid option: $1"
        echo "Use: frontend, librechat-backend, go-api, go-proxy, or all"
        exit 1
        ;;
esac

