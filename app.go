package main

import (
	"byto/internal/builder"
	"byto/internal/command"
	"byto/internal/domain"
	"byto/internal/queue"
	"byto/internal/updater"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	queue         *queue.Queue
	settings      *domain.Setting
	mediaDefaults *domain.MediaDefaults
	updater       *updater.Updater
}

func NewApp() *App {
	return &App{
		queue:         queue.NewQueue(),
		settings:      domain.NewSetting(),
		mediaDefaults: domain.NewMediaDefaults(),
		updater:       updater.NewUpdater(),
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

func (a *App) UpdateSettings(parallelDownloads int) {
	a.settings.Update(parallelDownloads)
	log.Printf("Settings updated in memory: parallel=%d", parallelDownloads)
}

func (a *App) SaveSettings() error {
	log.Println("Saving settings to file")
	return a.settings.Save()
}

func (a *App) SelectDownloadFolder() string {
	return a.SelectDownloadFolderWithDefault("")
}

func (a *App) SelectDownloadFolderWithDefault(defaultPath string) string {
	if defaultPath == "" {
		defaultPath = a.mediaDefaults.DownloadPath
	}
	// Check if the path exists, fallback to Downloads if not
	if defaultPath != "" {
		if _, err := os.Stat(defaultPath); os.IsNotExist(err) {
			home, _ := os.UserHomeDir()
			defaultPath = filepath.Join(home, "Downloads")
			log.Printf("Saved path doesn't exist, falling back to: %s", defaultPath)
		}
	}
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Select Download Folder",
		DefaultDirectory: defaultPath,
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
	if a.mediaDefaults != nil {
		return a.mediaDefaults.DownloadPath
	}
	return "./downloads"
}

// GetMediaDefaults returns the current media defaults (quality and download path)
func (a *App) GetMediaDefaults() *domain.MediaDefaults {
	return a.mediaDefaults
}

// UpdateMediaDefaults updates the media defaults for new items
func (a *App) UpdateMediaDefaults(quality string, downloadPath string) {
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
	a.mediaDefaults.Update(q, downloadPath)
	log.Printf("Media defaults updated in memory: quality=%s, path=%s", quality, downloadPath)
}

// SaveMediaDefaults saves the media defaults to file
func (a *App) SaveMediaDefaults() error {
	log.Println("Saving media defaults to file")
	return a.mediaDefaults.Save()
}

func (a *App) AddToQueue(url string, quality string, customPath string) string {
	id := uuid.New().String()
	log.Printf("Adding to queue: %s with id: %s", url, id)

	filePath := a.mediaDefaults.DownloadPath
	if customPath != "" {
		filePath = customPath
	}

	// Convert quality string to VideoQuality
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

	a.queue.Add(&domain.Media{
		ID:       id,
		URL:      url,
		Title:    "Pending...",
		FilePath: filePath,
		Quality:  q,
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
	a.PauseSingleDownload(id)
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
				title := "Pending..."
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

				// Initialize builder - use media's own FilePath and Quality
				b := builder.NewYTDLPBuilder().
					URL(m.URL).
					Quality(m.Quality).
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
		title := "Pending..."
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
			Quality(media.Quality).
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

func (a *App) GetAppVersion() string {
	return a.updater.GetAppVersion()
}

func (a *App) UpdateYTDLP() updater.UpdateResult {
	log.Println("Updating yt-dlp...")
	result := a.updater.UpdateYTDLP()
	log.Printf("yt-dlp update result: %s", result.Message)
	return result
}

func (a *App) CheckYtDlpUpdate() updater.UpdateResult {
	log.Println("Checking for yt-dlp updates...")
	result := a.updater.CheckYtDlpUpdate()
	log.Printf("yt-dlp update check: hasUpdate=%v, current=%s, latest=%s", result.HasUpdate, result.CurrentVersion, result.LatestVersion)
	return result
}

func (a *App) CheckYtDlp() updater.YtDlpStatus {
	log.Println("Checking yt-dlp installation...")
	status := a.updater.CheckYtDlp()
	log.Printf("yt-dlp status: installed=%v, path=%s, version=%s", status.Installed, status.Path, status.Version)
	return status
}

func (a *App) DownloadYtDlp() error {
	log.Println("Downloading yt-dlp...")

	progressCallback := func(downloaded, total int64) {
		var percentage float64
		if total > 0 {
			percentage = float64(downloaded) / float64(total) * 100
		}
		runtime.EventsEmit(a.ctx, "ytdlp_download_progress", map[string]interface{}{
			"downloaded": downloaded,
			"total":      total,
			"percentage": percentage,
		})
	}

	err := a.updater.DownloadYtDlp(progressCallback)
	if err != nil {
		log.Printf("Failed to download yt-dlp: %v", err)
		return err
	}

	log.Println("yt-dlp downloaded successfully")
	return nil
}

func (a *App) GetYtDlpPath() string {
	status := a.updater.CheckYtDlp()
	return status.Path
}

func (a *App) CheckAppUpdate() updater.UpdateResult {
	log.Println("Checking for app updates...")
	result := a.updater.CheckAppUpdate()
	if result.Success {
		log.Printf("App update check: current=%s, latest=%s, hasUpdate=%v",
			result.CurrentVersion, result.LatestVersion, result.HasUpdate)
	} else {
		log.Printf("App update check failed: %s", result.Message)
	}
	return result
}

func (a *App) DownloadAppUpdate(downloadURL string) (string, error) {
	log.Printf("Downloading app update from: %s", downloadURL)

	// Emit progress events
	progressCallback := func(downloaded, total int64) {
		var percentage float64
		if total > 0 {
			percentage = float64(downloaded) / float64(total) * 100
		}
		runtime.EventsEmit(a.ctx, "update_download_progress", map[string]interface{}{
			"downloaded": downloaded,
			"total":      total,
			"percentage": percentage,
		})
	}

	installerPath, err := a.updater.DownloadAppUpdate(downloadURL, progressCallback)
	if err != nil {
		log.Printf("Failed to download update: %v", err)
		return "", err
	}

	log.Printf("Update downloaded to: %s", installerPath)
	return installerPath, nil
}

func (a *App) LaunchInstaller(installerPath string) error {
	log.Printf("Launching installer: %s", installerPath)
	err := a.updater.LaunchInstaller(installerPath)
	if err != nil {
		log.Printf("Failed to launch installer: %v", err)
		return err
	}
	a.ShutDown()
	return nil
}

func (a *App) PerformFullUpdate() map[string]interface{} {
	log.Println("Performing full update check...")

	// Step 1: Update yt-dlp
	runtime.EventsEmit(a.ctx, "update_status", map[string]interface{}{
		"step":    "ytdlp",
		"message": "Updating yt-dlp...",
	})
	ytdlpResult := a.updater.UpdateYTDLP()

	// Step 2: Check for app updates
	runtime.EventsEmit(a.ctx, "update_status", map[string]interface{}{
		"step":    "app_check",
		"message": "Checking for app updates...",
	})
	appResult := a.updater.CheckAppUpdate()

	return map[string]interface{}{
		"ytdlp": map[string]interface{}{
			"success": ytdlpResult.Success,
			"message": ytdlpResult.Message,
		},
		"app": map[string]interface{}{
			"success":         appResult.Success,
			"message":         appResult.Message,
			"current_version": appResult.CurrentVersion,
			"latest_version":  appResult.LatestVersion,
			"has_update":      appResult.HasUpdate,
			"changelog":       appResult.Changelog,
			"download_url":    appResult.DownloadURL,
		},
	}
}

func (a *App) CheckFfmpeg() updater.FfmpegStatus {
	log.Println("Checking ffmpeg installation...")
	status := a.updater.CheckFfmpeg()
	log.Printf("ffmpeg status: installed=%v, path=%s, version=%s", status.Installed, status.Path, status.Version)
	return status
}

func (a *App) DownloadFfmpeg() error {
	log.Println("Downloading ffmpeg...")
	progressCallback := func(downloaded, total int64) {
		var percentage float64
		if total > 0 {
			percentage = float64(downloaded) / float64(total) * 100
		}
		runtime.EventsEmit(a.ctx, "ffmpeg_download_progress", map[string]interface{}{
			"downloaded": downloaded,
			"total":      total,
			"percentage": percentage,
		})
	}
	err := a.updater.DownloadFfmpeg(progressCallback)
	if err != nil {
		log.Printf("Failed to download ffmpeg: %v", err)
		return err
	}
	log.Println("ffmpeg downloaded successfully")
	return nil
}

func (a *App) ShutDown() {
	log.Println("Shutting down Byto App")
	runtime.Quit(a.ctx)
}
