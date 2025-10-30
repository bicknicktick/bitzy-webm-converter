package main

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/cors"
)

const (
	Port          = ":2424"
	MaxFileSize   = 100 * 1024 * 1024 // 100MB
	MaxConcurrent = 2                  // Max concurrent conversions (limited to prevent CPU overload)
	UploadDir     = "./web-uploads"
	OutputDir     = "./web-output"
	TempDir       = "./web-temp"
	MaxCPUUsage   = 70                 // Maximum CPU usage percentage
)

type Job struct {
	ID          string    `json:"id"`
	FileName    string    `json:"filename"`
	FileSize    int64     `json:"filesize"`
	OutputName  string    `json:"output_name"`
	Status      string    `json:"status"` // queued, processing, completed, failed
	Progress    int       `json:"progress"`
	QueuePos    int       `json:"queue_position"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Error       string    `json:"error,omitempty"`
	// For Telegram jobs
	TelegramChatID int64 `json:"-"`
	TelegramMsgID  int   `json:"-"`
}

type Queue struct {
	mu         sync.RWMutex
	jobs       []*Job
	processing map[string]*Job
	completed  map[string]*Job
	clients    map[*websocket.Conn]bool
}

var (
	queue = &Queue{
		jobs:       make([]*Job, 0),
		processing: make(map[string]*Job),
		completed:  make(map[string]*Job),
		clients:    make(map[*websocket.Conn]bool),
	}
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	telegramBot *tgbotapi.BotAPI
)

func main() {
	// Create directories
	os.MkdirAll(UploadDir, 0755)
	os.MkdirAll(OutputDir, 0755)
	os.MkdirAll(TempDir, 0755)
	os.MkdirAll("web", 0755)

	// Generate favicon
	GenerateFavicon()

	// Initialize Telegram bot if token provided
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramToken != "" {
		initTelegramBot(telegramToken)
	}

	// Start queue processor
	go queueProcessor()

	// HTTP routes
	router := mux.NewRouter()
	
	// API routes
	router.HandleFunc("/api/upload", handleUpload).Methods("POST")
	router.HandleFunc("/api/jobs", handleGetJobs).Methods("GET")
	router.HandleFunc("/api/jobs/{id}", handleGetJob).Methods("GET")
	router.HandleFunc("/api/jobs/{id}/download", handleDownload).Methods("GET")
	router.HandleFunc("/api/jobs/download-all", handleDownloadAll).Methods("POST")
	router.HandleFunc("/ws", handleWebSocket)
	
	// Static files
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/")))
	
	// CORS middleware
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(router)
	
	log.Printf("Server starting on http://localhost%s", Port)
	if telegramBot != nil {
		log.Printf("Telegram bot enabled: @%s", telegramBot.Self.UserName)
	}
	
	log.Fatal(http.ListenAndServe(Port, handler))
}

// Telegram Bot Functions
func initTelegramBot(token string) {
	var err error
	telegramBot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("Failed to initialize Telegram bot: %v", err)
		return
	}
	
	log.Printf("Telegram bot initialized: @%s", telegramBot.Self.UserName)
	go handleTelegramUpdates()
}

func handleTelegramUpdates() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	
	updates := telegramBot.GetUpdatesChan(u)
	
	for update := range updates {
		if update.Message == nil {
			continue
		}
		
		go processTelegramMessage(update.Message)
	}
}

func processTelegramMessage(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	
	// Handle commands
	if message.IsCommand() {
		switch message.Command() {
		case "start":
			text := "🎥 *WebM to MP4 Converter*\n\nSend me a WebM file and I'll convert it to MP4!"
			msg := tgbotapi.NewMessage(chatID, text)
			msg.ParseMode = "Markdown"
			telegramBot.Send(msg)
		case "status":
			queue.mu.RLock()
			q := len(queue.jobs)
			p := len(queue.processing)
			queue.mu.RUnlock()
			text := fmt.Sprintf("📊 Queue: %d | Processing: %d/%d", q, p, MaxConcurrent)
			telegramBot.Send(tgbotapi.NewMessage(chatID, text))
		default:
			telegramBot.Send(tgbotapi.NewMessage(chatID, "Unknown command"))
		}
		return
	}
	
	// Handle document
	if message.Document != nil {
		handleTelegramDocument(message)
	}
}

func handleTelegramDocument(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	doc := message.Document
	
	// Check file extension
	if !strings.HasSuffix(strings.ToLower(doc.FileName), ".webm") {
		telegramBot.Send(tgbotapi.NewMessage(chatID, "❌ Please send a WebM file"))
		return
	}
	
	// Check size
	if doc.FileSize > MaxFileSize {
		telegramBot.Send(tgbotapi.NewMessage(chatID, "❌ File too large (max 100MB)"))
		return
	}
	
	// Send processing message
	msg := tgbotapi.NewMessage(chatID, "⏳ Processing...")
	sentMsg, _ := telegramBot.Send(msg)
	
	// Download file
	file, err := telegramBot.GetFile(tgbotapi.FileConfig{FileID: doc.FileID})
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, "❌ Failed to download")
		telegramBot.Send(editMsg)
		return
	}
	
	// Download to temp
	tempPath := filepath.Join(TempDir, fmt.Sprintf("tg_%d_%s", chatID, doc.FileName))
	fileURL := file.Link(telegramBot.Token)
	
	if err := downloadFileFromURL(fileURL, tempPath); err != nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, "❌ Download failed")
		telegramBot.Send(editMsg)
		return
	}
	
	// Create job
	job := &Job{
		ID:             uuid.New().String(),
		FileName:       doc.FileName,
		FileSize:       int64(doc.FileSize),
		OutputName:     strings.TrimSuffix(doc.FileName, ".webm") + ".mp4",
		Status:         "queued",
		CreatedAt:      time.Now(),
		TelegramChatID: chatID,
		TelegramMsgID:  sentMsg.MessageID,
	}
	
	// Move to upload dir
	uploadPath := filepath.Join(UploadDir, job.ID+"_"+job.FileName)
	os.Rename(tempPath, uploadPath)
	
	// Add to queue
	queue.mu.Lock()
	queue.jobs = append(queue.jobs, job)
	for i, j := range queue.jobs {
		j.QueuePos = i + 1
	}
	queue.mu.Unlock()
	
	// Update message
	editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, 
		fmt.Sprintf("📥 Added to queue #%d", job.QueuePos))
	telegramBot.Send(editMsg)
	
	// Monitor job
	go monitorTelegramJob(job)
}

func monitorTelegramJob(job *Job) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		queue.mu.RLock()
		
		// Check if completed
		if completed, ok := queue.completed[job.ID]; ok {
			queue.mu.RUnlock()
			
			// Send file
			outputPath := filepath.Join(OutputDir, job.ID+"_"+job.OutputName)
			sendTelegramFile(job.TelegramChatID, job.TelegramMsgID, outputPath, completed.OutputName)
			return
		}
		
		// Check if processing
		if processing, ok := queue.processing[job.ID]; ok {
			queue.mu.RUnlock()
			
			// Update progress
			editMsg := tgbotapi.NewEditMessageText(job.TelegramChatID, job.TelegramMsgID,
				fmt.Sprintf("🔄 Converting... %d%%", processing.Progress))
			telegramBot.Send(editMsg)
			continue
		}
		
		queue.mu.RUnlock()
		
		// Check timeout
		if time.Since(job.CreatedAt) > 30*time.Minute {
			editMsg := tgbotapi.NewEditMessageText(job.TelegramChatID, job.TelegramMsgID, "❌ Timeout")
			telegramBot.Send(editMsg)
			return
		}
	}
}

func sendTelegramFile(chatID int64, msgID int, filepath, filename string) {
	// Read file
	data, err := os.ReadFile(filepath)
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, msgID, "❌ Failed to send file")
		telegramBot.Send(editMsg)
		return
	}
	
	// Update message
	editMsg := tgbotapi.NewEditMessageText(chatID, msgID, "📤 Uploading...")
	telegramBot.Send(editMsg)
	
	// Send document
	doc := tgbotapi.FileBytes{
		Name:  filename,
		Bytes: data,
	}
	
	msg := tgbotapi.NewDocument(chatID, doc)
	msg.Caption = "✅ Converted successfully!"
	telegramBot.Send(msg)
	
	// Final update
	editMsg = tgbotapi.NewEditMessageText(chatID, msgID, 
		fmt.Sprintf("✅ Done! File: %s", filename))
	telegramBot.Send(editMsg)
}

// Download helper
func downloadFileFromURL(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()
	
	_, err = io.Copy(out, resp.Body)
	return err
}

// Web Server Handlers
func handleUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(MaxFileSize)
	
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Check file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".webm") {
		http.Error(w, "Only WebM files are allowed", http.StatusBadRequest)
		return
	}
	
	// Get rename option
	renameOption := r.FormValue("rename")
	customName := r.FormValue("custom_name")
	outputName := getOutputName(header.Filename, renameOption, customName)
	
	// Create job
	job := &Job{
		ID:         uuid.New().String(),
		FileName:   header.Filename,
		FileSize:   header.Size,
		OutputName: outputName,
		Status:     "queued",
		CreatedAt:  time.Now(),
	}
	
	// Save file
	uploadPath := filepath.Join(UploadDir, job.ID+"_"+header.Filename)
	dst, err := os.Create(uploadPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Add to queue
	queue.mu.Lock()
	queue.jobs = append(queue.jobs, job)
	for i, j := range queue.jobs {
		j.QueuePos = i + 1
	}
	queue.mu.Unlock()
	
	// Broadcast update
	broadcastUpdate(job)
	
	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func handleGetJobs(w http.ResponseWriter, r *http.Request) {
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	
	allJobs := make([]*Job, 0)
	
	// Add all jobs from different states
	allJobs = append(allJobs, queue.jobs...)
	for _, job := range queue.processing {
		allJobs = append(allJobs, job)
	}
	for _, job := range queue.completed {
		allJobs = append(allJobs, job)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allJobs)
}

func handleGetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	
	// Check all states
	for _, job := range queue.jobs {
		if job.ID == jobID {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(job)
			return
		}
	}
	
	if job, exists := queue.processing[jobID]; exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
		return
	}
	
	if job, exists := queue.completed[jobID]; exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(job)
		return
	}
	
	http.Error(w, "Job not found", http.StatusNotFound)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	queue.mu.RLock()
	job, exists := queue.completed[jobID]
	queue.mu.RUnlock()
	
	if !exists {
		http.Error(w, "Job not found or not completed", http.StatusNotFound)
		return
	}
	
	outputPath := filepath.Join(OutputDir, job.ID+"_"+job.OutputName)
	
	// Check if file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	
	// Set headers
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", job.OutputName))
	
	// Serve file
	http.ServeFile(w, r, outputPath)
}

func handleDownloadAll(w http.ResponseWriter, r *http.Request) {
	var request struct {
		JobIDs []string `json:"job_ids"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// Create temp zip file
	tempZip := filepath.Join(TempDir, fmt.Sprintf("download_%d.zip", time.Now().Unix()))
	zipFile, err := os.Create(tempZip)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempZip)
	defer zipFile.Close()
	
	// Create zip writer
	zipWriter := zip.NewWriter(zipFile)
	
	queue.mu.RLock()
	for _, jobID := range request.JobIDs {
		if job, exists := queue.completed[jobID]; exists {
			outputPath := filepath.Join(OutputDir, job.ID+"_"+job.OutputName)
			
			// Add file to zip
			if fileData, err := os.ReadFile(outputPath); err == nil {
				if w, err := zipWriter.Create(job.OutputName); err == nil {
					w.Write(fileData)
				}
			}
		}
	}
	queue.mu.RUnlock()
	
	zipWriter.Close()
	
	// Serve zip file
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"converted_videos.zip\"")
	http.ServeFile(w, r, tempZip)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	
	queue.mu.Lock()
	queue.clients[conn] = true
	queue.mu.Unlock()
	
	defer func() {
		queue.mu.Lock()
		delete(queue.clients, conn)
		queue.mu.Unlock()
	}()
	
	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastUpdate(job *Job) {
	message := map[string]interface{}{
		"type": "job_update",
		"job":  job,
	}
	
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	
	for client := range queue.clients {
		err := client.WriteJSON(message)
		if err != nil {
			client.Close()
			delete(queue.clients, client)
		}
	}
}

// Processing Functions
func queueProcessor() {
	cpuMonitor := NewCPUMonitor()
	
	for {
		cpuUsage := cpuMonitor.GetCPUUsage()
		
		queue.mu.Lock()
		canProcess := len(queue.processing) < MaxConcurrent && len(queue.jobs) > 0
		
		if canProcess && cpuUsage > MaxCPUUsage {
			log.Printf("⚠️ CPU usage too high (%.1f%%), waiting...", cpuUsage)
			canProcess = false
		}
		
		if canProcess {
			job := queue.jobs[0]
			queue.jobs = queue.jobs[1:]
			queue.processing[job.ID] = job
			
			for i, j := range queue.jobs {
				j.QueuePos = i + 1
			}
			
			queue.mu.Unlock()
			
			log.Printf("🚀 Starting job %s (CPU: %.1f%%)", job.ID, cpuUsage)
			go processJob(job)
		} else {
			queue.mu.Unlock()
		}
		
		if cpuUsage > MaxCPUUsage {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}

func processJob(job *Job) {
	log.Printf("Processing job: %s", job.ID)
	
	job.Status = "processing"
	job.StartedAt = time.Now()
	job.Progress = 0
	broadcastUpdate(job)
	
	inputPath := filepath.Join(UploadDir, job.ID+"_"+job.FileName)
	outputPath := filepath.Join(OutputDir, job.ID+"_"+job.OutputName)
	
	duration, err := getVideoDuration(inputPath)
	if err != nil {
		log.Printf("Warning: Could not get duration: %v", err)
		duration = 0
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	
	err = convertVideoWithProgress(ctx, inputPath, outputPath, duration, func(progress float64) {
		job.Progress = int(progress)
		broadcastUpdate(job)
		
		// Update Telegram if it's a Telegram job
		if job.TelegramChatID != 0 && telegramBot != nil {
			// Rate limit updates
			if int(progress)%10 == 0 {
				editMsg := tgbotapi.NewEditMessageText(job.TelegramChatID, job.TelegramMsgID,
					fmt.Sprintf("🔄 Converting... %d%%", int(progress)))
				telegramBot.Send(editMsg)
			}
		}
	})
	
	if err != nil {
		log.Printf("First attempt failed for %s, trying fallback: %v", job.ID, err)
		err = fallbackConversion(ctx, inputPath, outputPath)
	}
	
	queue.mu.Lock()
	delete(queue.processing, job.ID)
	
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		log.Printf("Job %s failed: %v", job.ID, err)
	} else {
		job.Status = "completed"
		job.Progress = 100
		job.CompletedAt = time.Now()
		queue.completed[job.ID] = job
		log.Printf("Job %s completed in %s", job.ID, time.Since(job.StartedAt).Round(time.Second))
	}
	queue.mu.Unlock()
	
	broadcastUpdate(job)
	
	// Cleanup input
	os.Remove(inputPath)
	
	// Schedule output cleanup after 1 hour
	go func() {
		time.Sleep(1 * time.Hour)
		os.Remove(outputPath)
		
		queue.mu.Lock()
		delete(queue.completed, job.ID)
		queue.mu.Unlock()
	}()
}

// FFmpeg Functions
func getVideoDuration(filepath string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filepath)
	
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}
	
	return duration, nil
}

func convertVideoWithProgress(ctx context.Context, input, output string, duration float64, progressCallback func(float64)) error {
	cmd := exec.CommandContext(ctx, "nice", "-n", "10", "ffmpeg",
		"-i", input,
		"-threads", "2",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "28",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-max_muxing_queue_size", "9999",
		"-progress", "pipe:1",
		"-y", output)
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	
	if err := cmd.Start(); err != nil {
		return err
	}
	
	scanner := bufio.NewScanner(stdout)
	var currentTime float64
	
	for scanner.Scan() {
		line := scanner.Text()
		
		if strings.HasPrefix(line, "out_time_ms=") {
			timeStr := strings.TrimPrefix(line, "out_time_ms=")
			if timeMicros, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
				currentTime = float64(timeMicros) / 1000000.0
				
				if duration > 0 {
					progress := (currentTime / duration) * 100
					if progress > 100 {
						progress = 100
					}
					progressCallback(progress)
				}
			}
		} else if strings.HasPrefix(line, "progress=") {
			status := strings.TrimPrefix(line, "progress=")
			if status == "end" {
				progressCallback(100)
			}
		}
	}
	
	return cmd.Wait()
}

func fallbackConversion(ctx context.Context, input, output string) error {
	log.Printf("Running fallback conversion for %s", input)
	
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", input,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "28",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "44100",
		"-movflags", "+faststart",
		"-max_muxing_queue_size", "9999",
		"-y", output)
	
	return cmd.Run()
}

// Helper Functions
func getOutputName(filename, renameOption, customName string) string {
	base := strings.TrimSuffix(filename, ".webm")
	
	switch renameOption {
	case "custom":
		if customName != "" {
			return customName + ".mp4"
		}
		return base + ".mp4"
	case "prefix":
		return "converted_" + base + ".mp4"
	case "date":
		return time.Now().Format("2006-01-02") + "_" + base + ".mp4"
	default:
		return base + ".mp4"
	}
}

func getDefaultName(filename string) string {
	base := strings.TrimSuffix(filename, ".webm")
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s.mp4", base, timestamp)
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return name
}
