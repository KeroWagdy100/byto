package domain

import (
	"context"
	"sync"
)

type Media struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	TotalBytes int64            `json:"total_bytes"`
	URL        string           `json:"url"`
	FilePath   string           `json:"file_path"`
	Quality    VideoQuality     `json:"quality"`
	OnlyAudio  bool             `json:"only_audio"`
	Status     DownloadStatus   `json:"status"`
	Progress   DownloadProgress `json:"progress"`
	mu         sync.Mutex
	// Context for cancellation
	Ctx        context.Context    `json:"-"`
	CancelFunc context.CancelFunc `json:"-"`

	OnProgress     func(id string, progress DownloadProgress) `json:"-"`
	OnStatusChange func(id string, status DownloadStatus)     `json:"-"`
	OnTitleChange  func(id string, title string)              `json:"-"`
}

type DownloadProgress struct {
	Percentage      int      `json:"percentage"`
	DownloadedBytes int64    `json:"downloaded_bytes"`
	Logs            []string `json:"logs"`
}

func (m *Media) AppendLog(log string) {
	m.mu.Lock()
	m.Progress.Logs = append(m.Progress.Logs, log)
	progress := m.Progress
	id := m.ID
	onProgress := m.OnProgress
	m.mu.Unlock()

	// Emit progress update with new log
	if onProgress != nil {
		go onProgress(id, progress)
	}
}

func (m *Media) SetTitle(title string) {
	m.mu.Lock()
	m.Title = title
	id := m.ID
	onTitleChange := m.OnTitleChange
	m.mu.Unlock()

	if onTitleChange != nil {
		go onTitleChange(id, title)
	}
}

func (m *Media) UpdateProgress(downloaded, total int64, percentage int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Progress.DownloadedBytes = downloaded
	m.TotalBytes = total
	m.Progress.Percentage = percentage

	if m.OnProgress != nil {
		go m.OnProgress(m.ID, m.Progress)
	}
}

func (m *Media) SetStatus(status DownloadStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Status = status
	if m.OnStatusChange != nil {
		go m.OnStatusChange(m.ID, m.Status)
	}
}

func (m *Media) Cancel() {
	m.mu.Lock()
	cancelFunc := m.CancelFunc
	m.mu.Unlock()

	if cancelFunc != nil {
		cancelFunc()
	}
}
