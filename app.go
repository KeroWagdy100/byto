package main

import (
	"byto/internal/builder"
	"byto/internal/command"
	"byto/internal/domain"
	"byto/internal/queue"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	goRuntime "runtime"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	queue    *queue.Queue
	settings *domain.Setting
}

func NewApp() *App {
	return &App{
		queue:    queue.NewQueue(),
		settings: domain.NewSetting(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	log.Println("Byto App started")
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) GetSettings() *domain.Setting {
	return a.settings
}

func (a *App) UpdateSettings(quality string, parallelDownloads int, downloadPath string) {
	var q domain.VideoQuality
	switch quality {
	case "360p":
		q = domain.Quality360p
	case "480p":
		q = domain.Quality480p
	case "720p":
		q = domain.Quality720p
	case "1080p":
		q = domain.Quality1080p
	case "1440p":
		q = domain.Quality1440p
	case "2160p":
		q = domain.Quality2160p
	default:
		q = domain.Quality1080p
	}
	a.settings.Update(q, parallelDownloads, downloadPath)
	log.Printf("Settings updated: quality=%s, parallel=%d, path=%s", quality, parallelDownloads, downloadPath)
}

func (a *App) SelectDownloadFolder() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Download Folder",
	})
	if err != nil {
		log.Printf("Error selecting folder: %v", err)
		return ""
	}
	return path
}

func (a *App) ShowInFolder(filePath string) {
	log.Printf("ShowInFolder called with path: %s", filePath)
	var cmd *exec.Cmd
	switch goRuntime.GOOS {
	case "windows":
		// Check if it's a directory or file
		info, err := os.Stat(filePath)
		if err != nil {
			log.Printf("Error checking path: %v", err)
			return
		}
		if info.IsDir() {
			// Open the folder directly
			cmd = exec.Command("explorer", filePath)
		} else {
			// Select the file in explorer
			cmd = exec.Command("explorer", "/select,", filePath)
		}
	case "darwin":
		cmd = exec.Command("open", "-R", filePath)
	default: // Linux
		cmd = exec.Command("xdg-open", filePath)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Error opening folder: %v", err)
	}
}

func (a *App) GetDefaultDownloadPath() string {
	return "./downloads"
}

func (a *App) AddToQueue(url string) string {
	id := uuid.New().String()
	log.Printf("Adding to queue: %s with id: %s", url, id)
	a.queue.Add(&domain.Media{
		ID:       id,
		URL:      url,
		Title:    "Detecting...",
		FilePath: a.settings.DownloadPath,
		Status:   domain.Pending,
		Progress: domain.DownloadProgress{
			Percentage:      0,
			DownloadedBytes: 0,
			Logs:            []string{},
		},
	})
	return id
}

func (a *App) RemoveFromQueue(id string) error {
	log.Printf("Removing from queue: %s", id)
	return a.queue.Remove(id)
}

func (a *App) GetQueue() []*domain.Media {
	return a.queue.GetAll()
}

func (a *App) StartDownloads() {
	log.Println("Starting downloads")
	if a.settings == nil {
		a.settings = domain.NewSetting()
	}

	queueItems := a.queue.GetAll()
	semaphore := make(chan struct{}, a.settings.ParallelDownloads)

	// Collect pending/failed/paused items in order
	var pendingItems []*domain.Media
	for _, media := range queueItems {
		if media.Status == domain.Pending || media.Status == domain.Failed || media.Status == domain.Paused {
			// Create a context for cancellation
			ctx, cancelFunc := context.WithCancel(context.Background())
			media.Ctx = ctx
			media.CancelFunc = cancelFunc

			// Attach callbacks - capture the media ID to avoid closure issues
			mediaID := media.ID
			media.OnProgress = func(id string, progress domain.DownloadProgress) {
				// Get the current media state to include title
				currentMedia, err := a.queue.Get(id)
				title := "Detecting..."
				totalBytes := int64(0)
				if err == nil && currentMedia != nil {
					title = currentMedia.Title
					totalBytes = currentMedia.TotalBytes
				}
				runtime.EventsEmit(a.ctx, "download_progress", map[string]interface{}{
					"id":          id,
					"title":       title,
					"total_bytes": totalBytes,
					"progress":    progress,
				})
			}

			media.OnStatusChange = func(id string, status domain.DownloadStatus) {
				runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
					"id":     id,
					"status": status,
				})
			}

			media.OnTitleChange = func(id string, title string) {
				runtime.EventsEmit(a.ctx, "download_title", map[string]interface{}{
					"id":    id,
					"title": title,
				})
			}

			pendingItems = append(pendingItems, media)
			_ = mediaID // used in callbacks via closure
		}
	}

	// Use a job channel to maintain FIFO order
	jobs := make(chan *domain.Media, len(pendingItems))
	for _, media := range pendingItems {
		jobs <- media
	}
	close(jobs)

	// Start workers that pull from the job channel in order
	for i := 0; i < a.settings.ParallelDownloads; i++ {
		go func() {
			for m := range jobs {
				semaphore <- struct{}{}

				m.SetStatus(domain.InProgress)
				log.Printf("Processing item: %s", m.URL)

				// Initialize builder - use media's own FilePath, not current settings
				b := builder.NewYTDLPBuilder().
					URL(m.URL).
					Quality(a.settings.Quality).
					DownloadPath(m.FilePath).
					SafeFilenames()

				cmd := &command.DownloadCommand{
					Builder: b,
				}

				if err := cmd.Execute(m); err != nil {
					if err == context.Canceled {
						// Download was paused, set status to Paused
						m.SetStatus(domain.Paused)
						log.Printf("Download paused for %s", m.URL)
					} else {
						m.SetStatus(domain.Failed)
						log.Printf("Download failed for %s: %v", m.URL, err)
						m.AppendLog(fmt.Sprintf("Download failed: %v", err))
					}
				} else {
					log.Printf("Download completed: %s", m.URL)
				}

				<-semaphore
			}
		}()
	}
}

func (a *App) PauseDownloads() {
	log.Println("Pausing all downloads")
	queueItems := a.queue.GetAll()

	for _, media := range queueItems {
		if media.Status == domain.InProgress {
			media.Cancel()
		}
	}
}

func (a *App) StartSingleDownload(id string) {
	log.Printf("Starting single download: %s", id)
	media, err := a.queue.Get(id)
	if err != nil {
		log.Printf("Error getting media from queue: %v", err)
		return
	}

	if media.Status != domain.Pending && media.Status != domain.Failed && media.Status != domain.Paused {
		log.Printf("Media %s is not in a startable state (status: %d)", id, media.Status)
		return
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	media.Ctx = ctx
	media.CancelFunc = cancelFunc

	media.OnProgress = func(id string, progress domain.DownloadProgress) {
		currentMedia, err := a.queue.Get(id)
		title := "Detecting..."
		totalBytes := int64(0)
		if err == nil && currentMedia != nil {
			title = currentMedia.Title
			totalBytes = currentMedia.TotalBytes
		}
		runtime.EventsEmit(a.ctx, "download_progress", map[string]interface{}{
			"id":          id,
			"title":       title,
			"total_bytes": totalBytes,
			"progress":    progress,
		})
	}

	media.OnStatusChange = func(id string, status domain.DownloadStatus) {
		runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
			"id":     id,
			"status": status,
		})
	}

	media.OnTitleChange = func(id string, title string) {
		runtime.EventsEmit(a.ctx, "download_title", map[string]interface{}{
			"id":    id,
			"title": title,
		})
	}

	go func() {
		media.SetStatus(domain.InProgress)
		log.Printf("Processing item: %s", media.URL)

		b := builder.NewYTDLPBuilder().
			URL(media.URL).
			Quality(a.settings.Quality).
			DownloadPath(media.FilePath).
			SafeFilenames()

		cmd := &command.DownloadCommand{
			Builder: b,
		}

		if err := cmd.Execute(media); err != nil {
			if err == context.Canceled {
				media.SetStatus(domain.Paused)
				log.Printf("Download paused for %s", media.URL)
			} else {
				media.SetStatus(domain.Failed)
				log.Printf("Download failed for %s: %v", media.URL, err)
				media.AppendLog(fmt.Sprintf("Download failed: %v", err))
			}
		} else {
			log.Printf("Download completed: %s", media.URL)
		}
	}()
}

func (a *App) PauseSingleDownload(id string) {
	log.Printf("Pausing single download: %s", id)
	media, err := a.queue.Get(id)
	if err != nil {
		log.Printf("Error getting media from queue: %v", err)
		return
	}

	if media.Status == domain.InProgress {
		media.Cancel()
	}
}
