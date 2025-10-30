#!/bin/bash

echo "========================================="
echo "  BITZY WEBM CONVERTER"
echo "  Complete Edition with Telegram Bot"
echo "========================================="
echo ""

# Check dependencies
echo "Checking dependencies..."

if ! command -v ffmpeg &> /dev/null; then
    echo "❌ FFmpeg not installed"
    echo "Install with: sudo apt-get install ffmpeg"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "❌ Go not installed"
    echo "Install from: https://golang.org/dl/"
    exit 1
fi

# Create required directories
mkdir -p web-uploads web-output web-temp web

# Kill existing instances
pkill -f webm2mp4-server 2>/dev/null
pkill -f main-server 2>/dev/null

# Check for Telegram bot token
if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo ""
    echo "⚠️  No Telegram bot token found"
    echo ""
    echo "To enable Telegram bot, set environment variable:"
    echo "export TELEGRAM_BOT_TOKEN='your_bot_token_here'"
    echo ""
    echo "Or create a bot:"
    echo "1. Message @BotFather on Telegram"
    echo "2. Send /newbot and follow instructions"
    echo "3. Copy the token and run:"
    echo "   export TELEGRAM_BOT_TOKEN='your_token'"
    echo ""
    read -p "Continue without Telegram bot? (y/n): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Setup cancelled"
        exit 1
    fi
else
    echo "✅ Telegram bot token found"
fi

# Install Go dependencies
echo ""
echo "Installing dependencies..."
go mod init webm2mp4 2>/dev/null || true
go get github.com/google/uuid
go get github.com/gorilla/mux
go get github.com/gorilla/websocket
go get github.com/rs/cors
go get github.com/go-telegram-bot-api/telegram-bot-api/v5

# Build the server
echo "Building server..."
go build -o webm2mp4-server main-server.go cpu-monitor.go generate-favicon.go

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

# Clear test files
rm -f test-*.webm test-*.mp4 speed-test-*.webm cpu-test-*.webm downloaded_test.mp4 2>/dev/null

# Display configuration
echo ""
echo "========================================="
echo "  STARTING SERVER"
echo "========================================="
echo ""
echo "Configuration:"
echo "  • Web Interface: http://localhost:8080"
echo "  • Max file size: 100MB"
echo "  • Concurrent jobs: 2"
echo "  • CPU limit: 70%"
echo "  • FFmpeg preset: ultrafast"

if [ ! -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "  • Telegram bot: ENABLED"
else
    echo "  • Telegram bot: DISABLED"
fi

echo ""
echo "Features:"
echo "  ✅ Web drag & drop interface"
echo "  ✅ Real-time progress tracking"
echo "  ✅ Download all (zip)"
echo "  ✅ CPU optimization"
echo "  ✅ Fallback conversion"

if [ ! -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "  ✅ Telegram bot support"
fi

echo ""
echo "========================================="
echo ""
echo "Press Ctrl+C to stop"
echo ""

# Run the server
./webm2mp4-server
