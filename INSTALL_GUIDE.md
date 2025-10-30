# 🚀 Installation Guide - BITZY WEBM Converter

## ⚡ Quick Start (3 Minutes)

### **Prerequisites**
- **Go** 1.21+ ([Download](https://go.dev/dl/))
- **FFmpeg** ([Install Guide](#ffmpeg-installation))
- **Git**

---

## 📦 **Step 1: Clone Repository**

```bash
git clone https://github.com/bicknicktick/bitzy-webm-converter.git
cd bitzy-webm-converter
```

---

## 🔧 **Step 2: Install Dependencies**

### **Install FFmpeg**

#### Ubuntu/Debian:
```bash
sudo apt update
sudo apt install ffmpeg -y
```

#### MacOS:
```bash
brew install ffmpeg
```

#### Windows:
Download from [ffmpeg.org](https://ffmpeg.org/download.html)

### **Install Go Dependencies**
```bash
go mod init webm2mp4
go get github.com/google/uuid
go get github.com/gorilla/mux
go get github.com/gorilla/websocket
go get github.com/rs/cors
go get github.com/go-telegram-bot-api/telegram-bot-api/v5
```

---

## 🏃 **Step 3: Run the Application**

### **Option A: Quick Start (Web Only)**

```bash
# Build
go build -o webm2mp4-server main-server.go cpu-monitor.go generate-favicon.go

# Run
./webm2mp4-server
```

Open browser: **http://localhost:8080**

### **Option B: With Telegram Bot**

1. **Create Bot:**
   - Message [@BotFather](https://t.me/BotFather) on Telegram
   - Send `/newbot` and follow instructions
   - Copy the token

2. **Set Token & Run:**
```bash
# Linux/Mac
export TELEGRAM_BOT_TOKEN='your_bot_token_here'
./start-complete.sh

# Windows
set TELEGRAM_BOT_TOKEN=your_bot_token_here
start-complete.bat
```

---

## ✅ **Verification Steps**

### **1. Check Web Interface**
```bash
curl http://localhost:8080
# Should return HTML content
```

### **2. Test Conversion**
```bash
# Create test file
ffmpeg -f lavfi -i testsrc=duration=1:size=320x240:rate=30 test.webm

# Upload via curl
curl -X POST -F "file=@test.webm" -F "rename=default" http://localhost:8080/api/upload
```

### **3. Check Telegram Bot**
- Search your bot on Telegram
- Send `/start`
- Should receive welcome message

---

## 🐛 **Troubleshooting**

### **Error: "command not found: go"**
```bash
# Install Go from https://go.dev/dl/
# Add to PATH:
export PATH=$PATH:/usr/local/go/bin
```

### **Error: "ffmpeg not found"**
```bash
# Ubuntu/Debian
sudo apt install ffmpeg

# Check installation
ffmpeg -version
```

### **Port 8080 already in use**
```bash
# Find process
lsof -i :8080

# Kill process
kill -9 <PID>

# Or change port in main-server.go
const Port = ":8081"
```

### **Telegram bot not responding**
- Check token is correct
- Ensure bot is not used elsewhere
- Check internet connection
- Try regenerating token with BotFather

### **Blank web page**
- Clear browser cache (Ctrl+Shift+R)
- Check console for errors (F12)
- Ensure all files are in correct directories

---

## 📁 **File Structure Verification**

Ensure your directory structure matches:

```
bitzy-webm-converter/
├── main-server.go
├── cpu-monitor.go
├── generate-favicon.go
├── start-complete.sh
├── go.mod
├── go.sum
└── web/
    ├── index.html
    ├── style.css
    ├── app.js
    ├── favicon.svg
    └── favicon.png
```

---

## 🚢 **Production Deployment**

### **Using PM2**
```bash
# Install PM2
npm install -g pm2

# Start with PM2
pm2 start ./webm2mp4-server --name bitzy-webm

# Auto-restart on reboot
pm2 startup
pm2 save
```

### **Using systemd (Linux)**

Create `/etc/systemd/system/bitzy-webm.service`:

```ini
[Unit]
Description=BITZY WEBM Converter
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/bitzy-webm
ExecStart=/opt/bitzy-webm/webm2mp4-server
Restart=on-failure
Environment="TELEGRAM_BOT_TOKEN=your_token_here"

[Install]
WantedBy=multi-user.target
```

Enable service:
```bash
sudo systemctl enable bitzy-webm
sudo systemctl start bitzy-webm
```

---

## ⚙️ **Configuration**

### **Environment Variables**
```bash
# Create .env file
cp .env.example .env

# Edit configuration
nano .env
```

### **Key Settings:**
- `PORT`: Server port (default: 8080)
- `MAX_FILE_SIZE`: Max upload size (default: 100MB)
- `MAX_CONCURRENT`: Parallel conversions (default: 2)
- `CPU_LIMIT`: CPU usage limit (default: 70%)
- `TELEGRAM_BOT_TOKEN`: Bot token (optional)

---

## 📊 **Performance Tuning**

### **For Faster Conversion:**
```bash
# Edit main-server.go
-preset ultrafast  # Fastest
-preset veryfast   # Fast
-preset fast       # Balanced
```

### **For Better Quality:**
```bash
# Edit main-server.go
-crf 23  # Higher quality, larger file
-crf 28  # Default
-crf 32  # Lower quality, smaller file
```

---

## 🔒 **Security**

### **Firewall (UFW)**
```bash
# Allow only necessary ports
sudo ufw allow 8080/tcp
sudo ufw enable
```

### **Nginx Reverse Proxy**
```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
    }
}
```

---

## 📝 **Testing**

### **Run Tests**
```bash
# Create test script
cat > test.sh << 'EOF'
#!/bin/bash
echo "Testing Web Interface..."
curl -s http://localhost:8080 > /dev/null && echo "✅ Web OK" || echo "❌ Web Failed"

echo "Testing API..."
curl -s http://localhost:8080/api/jobs > /dev/null && echo "✅ API OK" || echo "❌ API Failed"

echo "Testing WebSocket..."
curl -s http://localhost:8080/ws > /dev/null && echo "✅ WebSocket OK" || echo "❌ WebSocket Failed"
EOF

chmod +x test.sh
./test.sh
```

---

## 💝 **Support**

If you encounter issues:

1. Check this guide thoroughly
2. Check [GitHub Issues](https://github.com/bicknicktick/bitzy-webm-converter/issues)
3. Create new issue with:
   - Error message
   - Go version (`go version`)
   - FFmpeg version (`ffmpeg -version`)
   - Operating system
   - Steps to reproduce

---

## ✨ **Success Indicators**

You'll know everything is working when:

- ✅ Web interface loads at http://localhost:8080
- ✅ Gold theme with "BITZYWEBM" header visible
- ✅ Drag & drop works
- ✅ Files convert successfully
- ✅ Download works
- ✅ Telegram bot responds (if configured)

---

**Powered by [e.bitzy.id](https://e.bitzy.id)**

**Support Development:** [PayPal](https://paypal.me/bitzyid)
