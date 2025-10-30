class WebMConverter {
    constructor() {
        this.jobs = new Map();
        this.files = [];
        this.ws = null;
        this.reconnectAttempts = 0;
        
        this.initElements();
        this.initEventListeners();
        this.connectWebSocket();
        this.loadJobs();
    }
    
    initElements() {
        this.dropZone = document.getElementById('dropZone');
        this.fileInput = document.getElementById('fileInput');
        this.renameOptions = document.getElementById('renameOptions');
        this.customNameInput = document.getElementById('customNameInput');
        this.uploadBtn = document.getElementById('uploadBtn');
        this.jobsList = document.getElementById('jobsList');
        this.queueCount = document.getElementById('queueCount');
        this.processingCount = document.getElementById('processingCount');
        this.downloadAllBtn = document.getElementById('downloadAllBtn');
    }

    initEventListeners() {
        // Drop zone events
        this.dropZone.addEventListener('click', () => this.fileInput.click());
        this.dropZone.addEventListener('dragover', this.handleDragOver.bind(this));
        this.dropZone.addEventListener('dragleave', this.handleDragLeave.bind(this));
        this.dropZone.addEventListener('drop', this.handleDrop.bind(this));

        // File input
        this.fileInput.addEventListener('change', this.handleFileSelect.bind(this));

        document.querySelectorAll('input[name="rename"]').forEach(radio => {
            radio.addEventListener('change', this.handleRenameOptionChange.bind(this));
        });

        // Upload button
        this.uploadBtn.addEventListener('click', () => this.handleUpload());
        
        // Download All button
        if (this.downloadAllBtn) {
            this.downloadAllBtn.addEventListener('click', () => this.downloadAll());
        }
    }

    handleDragOver(e) {
        e.preventDefault();
        this.dropZone.classList.add('dragover');
    }

    handleDragLeave(e) {
        e.preventDefault();
        this.dropZone.classList.remove('dragover');
    }

    handleDrop(e) {
        e.preventDefault();
        this.dropZone.classList.remove('dragover');
        
        const files = Array.from(e.dataTransfer.files);
        this.handleFiles(files);
    }

    handleFileSelect(e) {
        const files = Array.from(e.target.files);
        this.handleFiles(files);
    }

    handleFiles(files) {
        const webmFiles = files.filter(file => file.name.toLowerCase().endsWith('.webm'));
        
        if (webmFiles.length === 0) {
            alert('Please select WebM files only');
            return;
        }

        const oversizedFiles = webmFiles.filter(file => file.size > 100 * 1024 * 1024);
        if (oversizedFiles.length > 0) {
            alert('Some files exceed 100MB limit');
            return;
        }

        this.files = webmFiles;
        this.renameOptions.style.display = 'block';
        
        // Update drop zone text
        const fileText = webmFiles.length === 1 
            ? webmFiles[0].name 
            : `${webmFiles.length} files selected`;
        this.dropZone.querySelector('.drop-text').textContent = fileText;
    }

    handleRenameOptionChange(e) {
        const value = e.target.value;
        this.customNameInput.style.display = value === 'custom' ? 'block' : 'none';
    }

    async handleUpload() {
        if (this.files.length === 0) return;

        this.uploadBtn.disabled = true;
        this.uploadBtn.textContent = 'Uploading...';

        const renameOption = document.querySelector('input[name="rename"]:checked').value;
        const customName = this.customNameInput.value;

        for (const file of this.files) {
            await this.uploadFile(file, renameOption, customName);
        }

        // Reset form
        this.files = [];
        this.fileInput.value = '';
        this.renameOptions.style.display = 'none';
        this.dropZone.querySelector('.drop-text').textContent = 'Drop WebM files here or click to browse';
        this.uploadBtn.disabled = false;
        this.uploadBtn.textContent = 'Start Conversion';
        this.customNameInput.value = '';
    }

    async uploadFile(file, renameOption, customName) {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('rename', renameOption);
        
        if (renameOption === 'custom' && customName) {
            // For multiple files with custom name, add index
            const name = this.files.length > 1 
                ? `${customName}_${this.files.indexOf(file) + 1}`
                : customName;
            formData.append('custom_name', name);
        }

        try {
            const response = await fetch('/api/upload', {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                throw new Error('Upload failed');
            }

            const job = await response.json();
            this.jobs.set(job.id, job);
            this.addJobToList(job);
        } catch (error) {
            console.error('Upload error:', error);
            alert(`Failed to upload ${file.name}`);
        }
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
        };

        this.ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            
            if (data.type === 'initial') {
                data.jobs.forEach(job => {
                    this.jobs.set(job.id, job);
                    this.addJobToList(job);
                });
            } else if (data.type === 'update') {
                this.updateJob(data.job);
            }
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            // Reconnect after 3 seconds
            setTimeout(() => this.connectWebSocket(), 3000);
        };
    }

    async loadJobs() {
        try {
            const response = await fetch('/api/jobs');
            const jobs = await response.json();
            
            jobs.forEach(job => {
                this.jobs.set(job.id, job);
                this.addJobToList(job);
            });
            
            this.updateStats();
        } catch (error) {
            console.error('Failed to load jobs:', error);
        }
    }

    addJobToList(job) {
        // Check if job already exists
        if (document.getElementById(`job-${job.id}`)) {
            this.updateJob(job);
            return;
        }

        const jobElement = document.createElement('div');
        jobElement.className = `job-item ${job.status === 'processing' ? 'processing' : ''}`;
        jobElement.id = `job-${job.id}`;
        jobElement.innerHTML = this.renderJob(job);
        
        // Insert based on status priority
        const firstCompleted = Array.from(this.jobsList.children).find(
            child => child.querySelector('.status-completed, .status-failed')
        );
        
        if (job.status === 'completed' || job.status === 'failed') {
            this.jobsList.appendChild(jobElement);
        } else if (firstCompleted) {
            this.jobsList.insertBefore(jobElement, firstCompleted);
        } else {
            this.jobsList.appendChild(jobElement);
        }
        
        this.updateStats();
    }

    updateJob(job) {
        this.jobs.set(job.id, job);
        
        const element = document.getElementById(`job-${job.id}`);
        if (element) {
            // Update classes
            element.className = `job-item ${job.status === 'processing' ? 'processing' : ''}`;
            element.innerHTML = this.renderJob(job);
        } else {
            this.addJobToList(job);
        }
        
        this.updateStats();
    }

    renderJob(job) {
        const status = this.getStatusDisplay(job.status);
        const size = this.formatFileSize(job.filesize);
        const processingClass = job.status === 'processing' ? 'processing' : '';
        
        let html = `
            <div class="job-header">
                <span class="job-name" title="${job.filename}">${job.filename}</span>
                <span class="job-status status-${job.status}">${status}</span>
            </div>
            <div class="job-details">
                <div class="job-info">
                    <span title="${job.output_name}">Output: ${this.truncateFilename(job.output_name)}</span>
                    <span>${size}</span>
                    ${job.queue_position > 0 ? `<span>Queue: #${job.queue_position}</span>` : ''}
                </div>
            </div>
        `;

        if (job.status === 'processing') {
            const progressPercent = job.progress || 0;
            const timeElapsed = job.started_at ? this.getTimeElapsed(job.started_at) : '0s';
            
            html += `
                <div class="progress-container">
                    <div class="progress-header">
                        <span class="progress-label">Converting... ${timeElapsed}</span>
                        <span class="progress-percentage">${progressPercent}%</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${progressPercent}%"></div>
                    </div>
                </div>
            `;
        }

        if (job.status === 'completed') {
            const duration = job.started_at ? this.getTimeElapsed(job.started_at, job.completed_at) : '';
            html += `
                <a href="/api/jobs/${job.id}/download" class="download-btn">
                    <svg class="download-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                        <polyline points="7 10 12 15 17 10"></polyline>
                        <line x1="12" y1="15" x2="12" y2="3"></line>
                    </svg>
                    Download MP4 ${duration ? `(${duration})` : ''}
                </a>
            `;
        }

        if (job.error) {
            html += `<div class="error-message" style="color: var(--error); font-size: 0.75rem; margin-top: 0.5rem;">${job.error}</div>`;
        }

        return html;
    }

    getStatusDisplay(status) {
        const displays = {
            'queued': 'Queued',
            'processing': 'Processing',
            'completed': 'Completed',
            'failed': 'Failed'
        };
        return displays[status] || status;
    }

    formatFileSize(bytes) {
        if (bytes < 1024) return bytes + ' B';
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
        return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    }

    truncateFilename(filename, maxLength = 30) {
        if (filename.length <= maxLength) return filename;
        const ext = filename.substring(filename.lastIndexOf('.'));
        const nameLength = maxLength - ext.length - 3; // 3 for "..."
        return filename.substring(0, nameLength) + '...' + ext;
    }

    getTimeElapsed(startTime, endTime) {
        const start = new Date(startTime);
        const end = endTime ? new Date(endTime) : new Date();
        const elapsed = Math.floor((end - start) / 1000);
        
        if (elapsed < 60) return `${elapsed}s`;
        if (elapsed < 3600) {
            const minutes = Math.floor(elapsed / 60);
            const seconds = elapsed % 60;
            return `${minutes}m ${seconds}s`;
        }
        const hours = Math.floor(elapsed / 3600);
        const minutes = Math.floor((elapsed % 3600) / 60);
        return `${hours}h ${minutes}m`;
    }

    updateStats() {
        const queuedCount = Array.from(this.jobs.values()).filter(j => j.status === 'queued').length;
        const processingCount = Array.from(this.jobs.values()).filter(j => j.status === 'processing').length;
        const completedCount = Array.from(this.jobs.values()).filter(j => j.status === 'completed').length;
        
        this.queueCount.textContent = `${queuedCount} ${queuedCount === 1 ? 'file' : 'files'}`;
        this.processingCount.textContent = `${processingCount}/2 processing`;
        
        // Show/hide Download All button
        if (this.downloadAllBtn) {
            this.downloadAllBtn.style.display = completedCount > 0 ? 'inline-block' : 'none';
            if (completedCount > 0) {
                this.downloadAllBtn.textContent = `Download All (${completedCount})`;
            }
        }
        
        // Show empty state if no jobs
        if (this.jobs.size === 0) {
            this.jobsList.innerHTML = `
                <div class="empty-state">
                    <div class="empty-icon">⊡</div>
                    <p>No files in queue</p>
                </div>
            `;
        }
    }
    
    async downloadAll() {
        const completedJobs = Array.from(this.jobs.values()).filter(j => j.status === 'completed');
        
        if (completedJobs.length === 0) {
            alert('No completed files to download');
            return;
        }
        
        // If only one file, download directly
        if (completedJobs.length === 1) {
            window.location.href = `/api/jobs/${completedJobs[0].id}/download`;
            return;
        }
        
        // Multiple files - request zip
        try {
            const jobIds = completedJobs.map(j => j.id);
            const response = await fetch('/api/jobs/download-all', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ job_ids: jobIds })
            });
            
            if (response.ok) {
                const blob = await response.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = `converted_videos_${Date.now()}.zip`;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                window.URL.revokeObjectURL(url);
            } else {
                alert('Failed to download files');
            }
        } catch (error) {
            console.error('Download error:', error);
            alert('Failed to download files');
        }
    }
}

// Initialize app
document.addEventListener('DOMContentLoaded', () => {
    new WebMConverter();
});
