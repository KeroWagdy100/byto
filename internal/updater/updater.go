package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const AppVersion = "1.0.0"

const (
	GitHubOwner     = "OmarNaru1110"
	GitHubRepo      = "byto"
	YtDlpReleaseURL = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"
)

type VersionInfo struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	Changelog   string `json:"changelog"`
	Downloads   struct {
		Windows string `json:"windows"`
		Darwin  string `json:"darwin"`
		Linux   string `json:"linux"`
	} `json:"downloads"`
	MinVersion string `json:"min_version"`
}

type UpdateResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	CurrentVersion string `json:"current_version,omitempty"`
	LatestVersion  string `json:"latest_version,omitempty"`
	HasUpdate      bool   `json:"has_update,omitempty"`
	Changelog      string `json:"changelog,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
}

type YtDlpStatus struct {
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Version   string `json:"version"`
}

type Updater struct {
	httpClient *http.Client
	ytdlpPath  string
}

func NewUpdater() *Updater {
	// Create a transport with optimized settings for downloads
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true, // Faster for binary downloads
		MaxIdleConnsPerHost: 5,
	}

	return &Updater{
		httpClient: &http.Client{
			Timeout:   5 * time.Minute, // Longer timeout for large downloads
			Transport: transport,
		},
	}
}

func (u *Updater) GetAppVersion() string {
	return AppVersion
}

// returns the path to the yt-dlp executable
func (u *Updater) GetYtDlpPath() string {
	if u.ytdlpPath != "" {
		return u.ytdlpPath
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}
	appDir := filepath.Dir(execPath)

	ytdlpName := "yt-dlp"
	if runtime.GOOS == "windows" {
		ytdlpName = "yt-dlp.exe"
	}

	u.ytdlpPath = filepath.Join(appDir, ytdlpName)
	return u.ytdlpPath
}

func (u *Updater) CheckYtDlp() YtDlpStatus {
	bundledPath := u.GetYtDlpPath()
	if _, err := os.Stat(bundledPath); err == nil {
		version := u.getYtDlpVersion(bundledPath)
		return YtDlpStatus{
			Installed: true,
			Path:      bundledPath,
			Version:   version,
		}
	}

	// global check
	globalPath, err := exec.LookPath("yt-dlp")
	if err == nil {
		version := u.getYtDlpVersion(globalPath)
		return YtDlpStatus{
			Installed: true,
			Path:      globalPath,
			Version:   version,
		}
	}

	return YtDlpStatus{
		Installed: false,
		Path:      "",
		Version:   "",
	}
}

func (u *Updater) getYtDlpVersion(path string) string {
	cmd := exec.Command(path, "--version")
	hideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func (u *Updater) DownloadYtDlp(progressCallback func(downloaded, total int64)) error {
	resp, err := u.httpClient.Get(YtDlpReleaseURL)
	if err != nil {
		return fmt.Errorf("failed to check yt-dlp releases: %w", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %w", err)
	}

	var downloadURL string
	var assetName string

	switch runtime.GOOS {
	case "windows":
		assetName = "yt-dlp.exe"
	case "darwin":
		assetName = "yt-dlp_macos"
	default:
		assetName = "yt-dlp"
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("could not find yt-dlp download for %s", runtime.GOOS)
	}

	dlResp, err := u.httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download yt-dlp: %w", err)
	}
	defer dlResp.Body.Close()

	ytdlpPath := u.GetYtDlpPath()
	out, err := os.Create(ytdlpPath)
	if err != nil {
		return fmt.Errorf("failed to create yt-dlp file: %w", err)
	}
	defer out.Close()

	total := dlResp.ContentLength
	var downloaded int64

	// Use larger buffer for faster downloads (256KB)
	buf := make([]byte, 256*1024)
	for {
		n, err := dlResp.Body.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			downloaded += int64(n)
			if progressCallback != nil {
				progressCallback(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download interrupted: %w", err)
		}
	}

	// Make executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(ytdlpPath, 0755); err != nil {
			return fmt.Errorf("failed to make yt-dlp executable: %w", err)
		}
	}

	return nil
}

func (u *Updater) UpdateYTDLP() UpdateResult {
	status := u.CheckYtDlp()

	if !status.Installed {
		return UpdateResult{
			Success: false,
			Message: "yt-dlp is not installed",
		}
	}

	cmd := exec.Command(status.Path, "-U")
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		if strings.Contains(strings.ToLower(outputStr), "up to date") || strings.Contains(strings.ToLower(outputStr), "up-to-date") {
			return UpdateResult{
				Success: true,
				Message: "yt-dlp is already up to date",
			}
		}
		return UpdateResult{
			Success: false,
			Message: fmt.Sprintf("Failed to update yt-dlp: %v\nOutput: %s", err, outputStr),
		}
	}

	return UpdateResult{
		Success: true,
		Message: strings.TrimSpace(string(output)),
	}
}

func (u *Updater) CheckAppUpdate() UpdateResult {
	versionURL := fmt.Sprintf(
		"https://raw.githubusercontent.com/%s/%s/main/version.json",
		GitHubOwner, GitHubRepo,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return UpdateResult{
			Success:        false,
			Message:        fmt.Sprintf("Failed to create request: %v", err),
			CurrentVersion: AppVersion,
		}
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return UpdateResult{
			Success:        false,
			Message:        fmt.Sprintf("Failed to check for updates: %v", err),
			CurrentVersion: AppVersion,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateResult{
			Success:        false,
			Message:        fmt.Sprintf("Failed to fetch version info: HTTP %d", resp.StatusCode),
			CurrentVersion: AppVersion,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return UpdateResult{
			Success:        false,
			Message:        fmt.Sprintf("Failed to read response: %v", err),
			CurrentVersion: AppVersion,
		}
	}

	var versionInfo VersionInfo
	if err := json.Unmarshal(body, &versionInfo); err != nil {
		return UpdateResult{
			Success:        false,
			Message:        fmt.Sprintf("Failed to parse version info: %v", err),
			CurrentVersion: AppVersion,
		}
	}

	hasUpdate := compareVersions(versionInfo.Version, AppVersion) > 0

	var downloadURL string
	switch runtime.GOOS {
	case "windows":
		downloadURL = versionInfo.Downloads.Windows
	case "darwin":
		downloadURL = versionInfo.Downloads.Darwin
	default:
		downloadURL = versionInfo.Downloads.Linux
	}

	return UpdateResult{
		Success:        true,
		Message:        "Version check completed",
		CurrentVersion: AppVersion,
		LatestVersion:  versionInfo.Version,
		HasUpdate:      hasUpdate,
		Changelog:      versionInfo.Changelog,
		DownloadURL:    downloadURL,
	}
}

func (u *Updater) DownloadAppUpdate(downloadURL string, progressCallback func(downloaded, total int64)) (string, error) {
	if downloadURL == "" {
		return "", fmt.Errorf("no download URL provided")
	}

	resp, err := u.httpClient.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download update: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download: HTTP %d", resp.StatusCode)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	downloadsDir := filepath.Join(homeDir, "Downloads")

	filename := filepath.Base(downloadURL)
	if filename == "" || filename == "." {
		filename = "byto-update.exe"
	}
	destPath := filepath.Join(downloadsDir, filename)

	file, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	totalSize := resp.ContentLength
	var downloaded int64

	// Use larger buffer for faster downloads (256KB)
	buf := make([]byte, 256*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return "", fmt.Errorf("failed to write file: %v", writeErr)
			}
			downloaded += int64(n)
			if progressCallback != nil {
				progressCallback(downloaded, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to download: %v", err)
		}
	}

	return destPath, nil
}

func (u *Updater) LaunchInstaller(installerPath string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", installerPath)
	case "darwin":
		cmd = exec.Command("open", installerPath)
	default:
		cmd = exec.Command("xdg-open", installerPath)
	}

	return cmd.Start()
}

// compareVersions compares two semantic versions
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &num1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &num2)
		}

		if num1 > num2 {
			return 1
		}
		if num1 < num2 {
			return -1
		}
	}

	return 0
}
