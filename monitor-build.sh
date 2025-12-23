#!/bin/bash

# Build Monitor Script
# This script monitors memory usage during npm build

echo "==================================="
echo "LibreChat Build Monitor"
echo "==================================="
echo ""

# Check current memory status
echo "📊 Current Memory Status:"
free -h
echo ""

# Check swap status
echo "💾 Swap Status:"
swapon --show
echo ""

# Check disk space
echo "💿 Disk Space:"
df -h /
echo ""

# Show Node.js version and max memory
echo "🟢 Node.js Configuration:"
node --version
echo "Max Old Space Size: $(node -e "console.log(require('v8').getHeapStatistics().heap_size_limit / 1024 / 1024 + ' MB')")"
echo ""

# Function to monitor memory in background
monitor_memory() {
    local log_file="/tmp/build_memory_monitor.log"
    echo "Timestamp,Total_RAM_MB,Used_RAM_MB,Free_RAM_MB,Swap_Used_MB,Top_Process,Top_Process_MEM" > "$log_file"
    
    while true; do
        timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        mem_info=$(free -m | grep Mem)
        swap_info=$(free -m | grep Swap)
        
        total_ram=$(echo $mem_info | awk '{print $2}')
        used_ram=$(echo $mem_info | awk '{print $3}')
        free_ram=$(echo $mem_info | awk '{print $4}')
        swap_used=$(echo $swap_info | awk '{print $3}')
        
        top_process=$(ps aux --sort=-%mem | head -2 | tail -1)
        process_name=$(echo $top_process | awk '{print $11}')
        process_mem=$(echo $top_process | awk '{print $4}')
        
        echo "$timestamp,$total_ram,$used_ram,$free_ram,$swap_used,$process_name,$process_mem" >> "$log_file"
        
        sleep 2
    done
}

# Show menu
echo "==================================="
echo "Select an option:"
echo "==================================="
echo "1) Build with memory monitoring"
echo "2) Build (standard - 6GB limit)"
echo "3) Build (low memory - 4GB limit)"
echo "4) Just monitor (no build)"
echo "5) View previous build log"
echo "6) Exit"
echo ""
read -p "Enter choice [1-6]: " choice

case $choice in
    1)
        echo ""
        echo "🔍 Starting memory monitoring in background..."
        monitor_memory &
        MONITOR_PID=$!
        
        echo "🏗️  Starting build process..."
        echo ""
        cd /home/ec2-user/librechat_me/InstiLibreChat/client
        
        # Run build
        npm run build
        BUILD_EXIT_CODE=$?
        
        # Stop monitoring
        kill $MONITOR_PID 2>/dev/null
        
        echo ""
        echo "==================================="
        echo "Build completed with exit code: $BUILD_EXIT_CODE"
        echo "==================================="
        
        if [ -f /tmp/build_memory_monitor.log ]; then
            echo ""
            echo "📈 Memory Usage Summary:"
            echo "Max RAM used: $(awk -F',' 'NR>1 {print $3}' /tmp/build_memory_monitor.log | sort -n | tail -1) MB"
            echo "Max Swap used: $(awk -F',' 'NR>1 {print $5}' /tmp/build_memory_monitor.log | sort -n | tail -1) MB"
            echo ""
            echo "Full log saved to: /tmp/build_memory_monitor.log"
        fi
        ;;
    2)
        echo ""
        echo "🏗️  Starting standard build (6GB limit)..."
        cd /home/ec2-user/librechat_me/InstiLibreChat/client
        npm run build
        ;;
    3)
        echo ""
        echo "🏗️  Starting low memory build (4GB limit)..."
        cd /home/ec2-user/librechat_me/InstiLibreChat/client
        npm run build:low-mem
        ;;
    4)
        echo ""
        echo "🔍 Monitoring system resources (Press Ctrl+C to stop)..."
        watch -n 2 'free -h && echo "" && echo "Top Memory Consumers:" && ps aux --sort=-%mem | head -6'
        ;;
    5)
        if [ -f /tmp/build_memory_monitor.log ]; then
            echo ""
            echo "📊 Previous Build Memory Log:"
            cat /tmp/build_memory_monitor.log
        else
            echo ""
            echo "❌ No previous build log found"
        fi
        ;;
    6)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac

echo ""
echo "✅ Done!"

