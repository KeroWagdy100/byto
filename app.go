package main

import (
	"byto/internal/builder"
	"byto/internal/command"
	"byto/internal/domain"
	"byto/internal/queue"
	"context"
	"fmt"
	"log"

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

func (a *App) AddToQueue(url string) {
	log.Printf("Adding to queue: %s", url)
	a.queue.Add(&domain.Media{
		ID:     uuid.New().String(),
		URL:    url,
		Status: domain.Pending,
	})
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
		// Default settings if nil
		a.settings = &domain.Setting{
			Quality:           domain.Quality1080p,
			ParallelDownloads: 3,
			DownloadPath:      "./downloads",
		}
	}

	queueItems := a.queue.GetAll()
	semaphore := make(chan struct{}, a.settings.ParallelDownloads)

	for _, media := range queueItems {
		if media.Status == domain.Pending {
			// Attach callbacks
			media.OnProgress = func(id string, progress domain.DownloadProgress) {
				runtime.EventsEmit(a.ctx, "download_progress", map[string]interface{}{
					"id":       id,
					"progress": progress,
				})
			}

			media.OnStatusChange = func(id string, status domain.DownloadStatus) {
				runtime.EventsEmit(a.ctx, "download_status", map[string]interface{}{
					"id":     id,
					"status": status,
				})
			}

			go func(m *domain.Media) {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				m.SetStatus(domain.InProgress)
				log.Printf("Processing item: %s", m.URL)

				// Initialize builder
				b := builder.NewYTDLPBuilder().
					URL(m.URL).
					Quality(a.settings.Quality).
					DownloadPath(a.settings.DownloadPath)

				cmd := &command.DownloadCommand{
					Builder: b,
				}

				if err := cmd.Execute(m); err != nil {
					m.SetStatus(domain.Failed)
					log.Printf("Download failed for %s: %v", m.URL, err)
					m.AppendLog(fmt.Sprintf("Download failed: %v", err))
				} else {
					log.Printf("Download completed: %s", m.URL)
				}
			}(media)
		}
	}
}
