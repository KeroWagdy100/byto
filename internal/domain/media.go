package domain

import "sync"

type Media struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	TotalBytes int64            `json:"total_bytes"`
	URL        string           `json:"url"`
	Status     DownloadStatus   `json:"status"`
	Progress   DownloadProgress `json:"progress"`
	mu         sync.Mutex
	// Callbacks for real-time updates
	OnProgress     func(id string, progress DownloadProgress) `json:"-"`
	OnStatusChange func(id string, status DownloadStatus)     `json:"-"`
}

type DownloadProgress struct {
	Percentage      int      `json:"percentage"`
	DownloadedBytes int64    `json:"downloaded_bytes"`
	Logs            []string `json:"logs"`
}

func (m *Media) AppendLog(log string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Progress.Logs = append(m.Progress.Logs, log)
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
