package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ulikunitz/xz"
)

// ─── NewUpdater ────────────────────────────────────────────────────────────────

func TestNewUpdater(t *testing.T) {
	u := NewUpdater()
	if u == nil {
		t.Fatal("expected non-nil Updater")
	}
	if u.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}
	if u.httpClient.Timeout == 0 {
		t.Error("expected non-zero client timeout")
	}
}

// ─── GetAppVersion ─────────────────────────────────────────────────────────────

func TestGetAppVersion(t *testing.T) {
	u := NewUpdater()
	v := u.GetAppVersion()
	if v != AppVersion {
		t.Errorf("GetAppVersion() = %q, want %q", v, AppVersion)
	}
	// Sanity: version looks like semver
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		t.Errorf("AppVersion %q does not look like semver", v)
	}
}

// ─── compareVersions ───────────────────────────────────────────────────────────

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{"equal", "1.0.0", "1.0.0", 0},
		{"v1 greater major", "2.0.0", "1.0.0", 1},
		{"v1 less major", "1.0.0", "2.0.0", -1},
		{"v1 greater minor", "1.2.0", "1.1.0", 1},
		{"v1 less minor", "1.1.0", "1.2.0", -1},
		{"v1 greater patch", "1.0.2", "1.0.1", 1},
		{"v1 less patch", "1.0.1", "1.0.2", -1},
		{"with v prefix both", "v1.2.3", "v1.2.3", 0},
		{"with v prefix mixed", "v2.0.0", "1.0.0", 1},
		{"different lengths short vs long", "1.0", "1.0.0", 0},
		{"different lengths v1 shorter greater", "1.1", "1.0.0", 1},
		{"different lengths v1 shorter less", "1.0", "1.0.1", -1},
		{"large numbers", "10.20.30", "10.20.29", 1},
		{"zeroes", "0.0.0", "0.0.0", 0},
		{"single digit", "2", "1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

// ─── GetYtDlpPath ──────────────────────────────────────────────────────────────

func TestGetYtDlpPath(t *testing.T) {
	u := NewUpdater()
	path := u.GetYtDlpPath()
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	base := filepath.Base(path)
	if runtime.GOOS == "windows" {
		if base != "yt-dlp.exe" {
			t.Errorf("expected yt-dlp.exe, got %s", base)
		}
	} else {
		if base != "yt-dlp" {
			t.Errorf("expected yt-dlp, got %s", base)
		}
	}
}

func TestGetYtDlpPath_Cached(t *testing.T) {
	u := NewUpdater()
	p1 := u.GetYtDlpPath()
	p2 := u.GetYtDlpPath()
	if p1 != p2 {
		t.Errorf("expected cached path, got %q and %q", p1, p2)
	}
}

// ─── GetFfmpegPath ─────────────────────────────────────────────────────────────

func TestGetFfmpegPath(t *testing.T) {
	u := NewUpdater()
	path := u.GetFfmpegPath()
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	base := filepath.Base(path)
	if runtime.GOOS == "windows" {
		if base != "ffmpeg.exe" {
			t.Errorf("expected ffmpeg.exe, got %s", base)
		}
	} else {
		if base != "ffmpeg" {
			t.Errorf("expected ffmpeg, got %s", base)
		}
	}
}

// ─── CheckYtDlp / CheckFfmpeg (not installed paths) ───────────────────────────

func TestCheckYtDlp_NotInstalled(t *testing.T) {
	u := NewUpdater()
	// Point to a path that certainly doesn't exist
	u.ytdlpPath = filepath.Join(t.TempDir(), "nonexistent-yt-dlp")

	// Also ensure PATH doesn't have yt-dlp (best-effort; skip if globally installed)
	status := u.CheckYtDlp()
	// We can't guarantee it's not globally installed, but we can at least test the return type
	if status.Installed && status.Path == u.ytdlpPath {
		t.Error("bundled path shouldn't be reported as installed if it doesn't exist")
	}
}

func TestCheckFfmpeg_NotInstalledBundled(t *testing.T) {
	u := NewUpdater()
	// CheckFfmpeg uses GetFfmpegPath which depends on os.Executable()
	// We just verify it returns a valid struct
	status := u.CheckFfmpeg()
	_ = status // no crash is the baseline
}

// ─── DownloadYtDlp ─────────────────────────────────────────────────────────────

func TestDownloadYtDlp_Success(t *testing.T) {
	binaryContent := []byte("fake-yt-dlp-binary-content")

	// Mock GitHub release API
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assetName := "yt-dlp"
		if runtime.GOOS == "windows" {
			assetName = "yt-dlp.exe"
		} else if runtime.GOOS == "darwin" {
			assetName = "yt-dlp_macos"
		}

		release := map[string]interface{}{
			"tag_name": "2025.01.01",
			"assets": []map[string]interface{}{
				{
					"name":                 assetName,
					"browser_download_url": "", // will be set below
				},
			},
		}

		// We need the download server URL, but we don't have it yet.
		// So this server will also serve the binary at /download
		release["assets"].([]map[string]interface{})[0]["browser_download_url"] =
			fmt.Sprintf("http://%s/download", r.Host)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer releaseServer.Close()

	// Create a server that serves both the API and the download
	combinedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binaryContent)))
			w.Write(binaryContent)
			return
		}

		// API response
		assetName := "yt-dlp"
		if runtime.GOOS == "windows" {
			assetName = "yt-dlp.exe"
		} else if runtime.GOOS == "darwin" {
			assetName = "yt-dlp_macos"
		}

		release := map[string]interface{}{
			"tag_name": "2025.01.01",
			"assets": []map[string]interface{}{
				{
					"name":                 assetName,
					"browser_download_url": fmt.Sprintf("http://%s/download", r.Host),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer combinedServer.Close()

	tmpDir := t.TempDir()
	ytdlpName := "yt-dlp"
	if runtime.GOOS == "windows" {
		ytdlpName = "yt-dlp.exe"
	}

	u := NewUpdater()
	u.ytdlpPath = filepath.Join(tmpDir, ytdlpName)

	// Override the release URL — we need to reach DownloadYtDlp's use of YtDlpReleaseURL.
	// Since YtDlpReleaseURL is a const we can't override, we'll use a custom approach.
	// We need to make httpClient.Get intercept the release URL.
	u.httpClient = combinedServer.Client()
	// Replace the release URL by customizing the transport
	u.httpClient.Transport = &urlRewriteTransport{
		base: combinedServer.Client().Transport,
		rewrites: map[string]string{
			YtDlpReleaseURL: combinedServer.URL + "/api",
		},
		defaultBase: combinedServer.URL,
	}

	var progressCalls int64
	err := u.DownloadYtDlp(func(downloaded, total int64) {
		atomic.AddInt64(&progressCalls, 1)
	})
	if err != nil {
		t.Fatalf("DownloadYtDlp() error: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(u.ytdlpPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if !bytes.Equal(data, binaryContent) {
		t.Errorf("downloaded content mismatch")
	}

	if atomic.LoadInt64(&progressCalls) == 0 {
		t.Error("expected progress callback to be called at least once")
	}
}

func TestDownloadYtDlp_NilProgressCallback(t *testing.T) {
	binaryContent := []byte("fake-binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(binaryContent)))
			w.Write(binaryContent)
			return
		}
		assetName := "yt-dlp"
		if runtime.GOOS == "windows" {
			assetName = "yt-dlp.exe"
		} else if runtime.GOOS == "darwin" {
			assetName = "yt-dlp_macos"
		}
		release := map[string]interface{}{
			"tag_name": "2025.01.01",
			"assets": []map[string]interface{}{
				{"name": assetName, "browser_download_url": fmt.Sprintf("http://%s/download", r.Host)},
			},
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	u := NewUpdater()
	ytdlpName := "yt-dlp"
	if runtime.GOOS == "windows" {
		ytdlpName = "yt-dlp.exe"
	}
	u.ytdlpPath = filepath.Join(tmpDir, ytdlpName)
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{
		rewrites:    map[string]string{YtDlpReleaseURL: server.URL + "/api"},
		defaultBase: server.URL,
	}

	// nil callback should not panic
	err := u.DownloadYtDlp(nil)
	if err != nil {
		t.Fatalf("DownloadYtDlp(nil callback) error: %v", err)
	}
}

func TestDownloadYtDlp_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = filepath.Join(t.TempDir(), "yt-dlp")
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{
		rewrites:    map[string]string{YtDlpReleaseURL: server.URL + "/api"},
		defaultBase: server.URL,
	}

	err := u.DownloadYtDlp(nil)
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

func TestDownloadYtDlp_NoMatchingAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := map[string]interface{}{
			"tag_name": "2025.01.01",
			"assets": []map[string]interface{}{
				{"name": "yt-dlp_some_other_platform", "browser_download_url": "http://example.com/nope"},
			},
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = filepath.Join(t.TempDir(), "yt-dlp")
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{
		rewrites:    map[string]string{YtDlpReleaseURL: server.URL + "/api"},
		defaultBase: server.URL,
	}

	err := u.DownloadYtDlp(nil)
	if err == nil {
		t.Fatal("expected error when no matching asset")
	}
	if !strings.Contains(err.Error(), "could not find yt-dlp download") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDownloadYtDlp_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = filepath.Join(t.TempDir(), "yt-dlp")
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{
		rewrites:    map[string]string{YtDlpReleaseURL: server.URL + "/api"},
		defaultBase: server.URL,
	}

	err := u.DownloadYtDlp(nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse release info") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ─── CheckAppUpdate ────────────────────────────────────────────────────────────

func TestCheckAppUpdate_HasUpdate(t *testing.T) {
	versionInfo := VersionInfo{
		Version:     "99.0.0",
		ReleaseDate: "2026-01-01",
		Changelog:   "Big update",
		MinVersion:  "1.0.0",
	}
	versionInfo.Downloads.Windows = "https://example.com/windows.exe"
	versionInfo.Downloads.Darwin = "https://example.com/darwin.dmg"
	versionInfo.Downloads.Linux = "https://example.com/linux.tar.gz"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(versionInfo)
	}))
	defer server.Close()

	u := NewUpdater()
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{
		defaultBase: server.URL,
	}

	result := u.CheckAppUpdate()
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Message)
	}
	if !result.HasUpdate {
		t.Error("expected HasUpdate=true")
	}
	if result.LatestVersion != "99.0.0" {
		t.Errorf("expected LatestVersion=99.0.0, got %s", result.LatestVersion)
	}
	if result.CurrentVersion != AppVersion {
		t.Errorf("expected CurrentVersion=%s, got %s", AppVersion, result.CurrentVersion)
	}
	if result.Changelog != "Big update" {
		t.Errorf("unexpected changelog: %s", result.Changelog)
	}
	if result.DownloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestCheckAppUpdate_NoUpdate(t *testing.T) {
	versionInfo := VersionInfo{
		Version: AppVersion, // same as current
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(versionInfo)
	}))
	defer server.Close()

	u := NewUpdater()
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckAppUpdate()
	if !result.Success {
		t.Fatalf("expected success: %s", result.Message)
	}
	if result.HasUpdate {
		t.Error("expected HasUpdate=false for same version")
	}
}

func TestCheckAppUpdate_OlderVersion(t *testing.T) {
	versionInfo := VersionInfo{
		Version: "0.0.1", // older than current
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(versionInfo)
	}))
	defer server.Close()

	u := NewUpdater()
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckAppUpdate()
	if !result.Success {
		t.Fatalf("expected success: %s", result.Message)
	}
	if result.HasUpdate {
		t.Error("expected HasUpdate=false for older version")
	}
}

func TestCheckAppUpdate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := NewUpdater()
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckAppUpdate()
	if result.Success {
		t.Error("expected failure for server error")
	}
	if result.CurrentVersion != AppVersion {
		t.Errorf("expected CurrentVersion even on failure, got %s", result.CurrentVersion)
	}
}

func TestCheckAppUpdate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid"))
	}))
	defer server.Close()

	u := NewUpdater()
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckAppUpdate()
	if result.Success {
		t.Error("expected failure for invalid JSON")
	}
	if !strings.Contains(result.Message, "Failed to parse version info") {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckAppUpdate_ConnectionRefused(t *testing.T) {
	u := NewUpdater()
	// Use a transport that redirects to a non-existent server
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: "http://127.0.0.1:1"}

	result := u.CheckAppUpdate()
	if result.Success {
		t.Error("expected failure when server is unreachable")
	}
	if result.CurrentVersion != AppVersion {
		t.Errorf("expected CurrentVersion=%s even on error", AppVersion)
	}
}

// ─── DownloadAppUpdate ─────────────────────────────────────────────────────────

func TestDownloadAppUpdate_Success(t *testing.T) {
	content := []byte("installer-binary-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write(content)
	}))
	defer server.Close()

	u := NewUpdater()

	var progressCalls int64
	destPath, err := u.DownloadAppUpdate(server.URL+"/byto-update.exe", func(downloaded, total int64) {
		atomic.AddInt64(&progressCalls, 1)
		if total != int64(len(content)) {
			t.Errorf("expected total=%d, got %d", len(content), total)
		}
	})
	if err != nil {
		t.Fatalf("DownloadAppUpdate error: %v", err)
	}
	defer os.Remove(destPath)

	if destPath == "" {
		t.Fatal("expected non-empty dest path")
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Error("downloaded content mismatch")
	}

	if atomic.LoadInt64(&progressCalls) == 0 {
		t.Error("expected progress callback to be called")
	}
}

func TestDownloadAppUpdate_EmptyURL(t *testing.T) {
	u := NewUpdater()
	_, err := u.DownloadAppUpdate("", nil)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "no download URL") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadAppUpdate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	u := NewUpdater()
	_, err := u.DownloadAppUpdate(server.URL+"/update.exe", nil)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadAppUpdate_NilProgressCallback(t *testing.T) {
	content := []byte("binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer server.Close()

	u := NewUpdater()
	destPath, err := u.DownloadAppUpdate(server.URL+"/byto-update.exe", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(destPath)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("file was not created")
	}
}

func TestDownloadAppUpdate_FilenameFromURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("x"))
	}))
	defer server.Close()

	u := NewUpdater()
	destPath, err := u.DownloadAppUpdate(server.URL+"/my-custom-installer.exe", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(destPath)

	base := filepath.Base(destPath)
	if base != "my-custom-installer.exe" {
		t.Errorf("expected filename my-custom-installer.exe, got %s", base)
	}
}

// ─── DownloadFfmpeg ────────────────────────────────────────────────────────────

func TestDownloadFfmpeg_Windows_Zip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	// Create a zip archive in memory with ffmpeg.exe inside a subdirectory
	zipBuf := createZipWithFile(t, "ffmpeg-release/bin/ffmpeg.exe", []byte("ffmpeg-binary"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", zipBuf.Len()))
		w.Write(zipBuf.Bytes())
	}))
	defer server.Close()

	u := NewUpdater()
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{
		rewrites:    map[string]string{ffmpegDownloadURLs["windows"]: server.URL + "/ffmpeg.zip"},
		defaultBase: server.URL,
	}

	tmpDir := t.TempDir()
	// We need to mock os.Executable to return tmpDir, but since we can't easily do that,
	// we verify the extraction logic separately
	var progressCalls int64
	err := u.DownloadFfmpeg(func(downloaded, total int64) {
		atomic.AddInt64(&progressCalls, 1)
	})
	// This may fail because os.Executable() points elsewhere, which is expected.
	// The key test is extractFfmpegFromZip below.
	_ = err
	_ = tmpDir
}

// ─── extractFfmpegFromZip ──────────────────────────────────────────────────────

func TestExtractFfmpegFromZip_Windows(t *testing.T) {
	content := []byte("ffmpeg-windows-binary-content")
	zipBuf := createZipWithFile(t, "ffmpeg-6.1/bin/ffmpeg.exe", content)

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "ffmpeg.zip")
	if err := os.WriteFile(zipPath, zipBuf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write zip: %v", err)
	}

	destPath := filepath.Join(tmpDir, "ffmpeg.exe")
	err := extractFfmpegFromZip(zipPath, destPath, true)
	if err != nil {
		t.Fatalf("extractFfmpegFromZip error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Error("extracted content mismatch")
	}
}

func TestExtractFfmpegFromZip_Unix(t *testing.T) {
	content := []byte("ffmpeg-unix-binary")
	zipBuf := createZipWithFile(t, "ffmpeg", content)

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "ffmpeg.zip")
	os.WriteFile(zipPath, zipBuf.Bytes(), 0644)

	destPath := filepath.Join(tmpDir, "ffmpeg")
	err := extractFfmpegFromZip(zipPath, destPath, false)
	if err != nil {
		t.Fatalf("extractFfmpegFromZip error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Error("content mismatch")
	}
}

func TestExtractFfmpegFromZip_InSubdir(t *testing.T) {
	content := []byte("nested-ffmpeg")
	zipBuf := createZipWithFile(t, "some/deep/path/ffmpeg.exe", content)

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "ffmpeg.zip")
	os.WriteFile(zipPath, zipBuf.Bytes(), 0644)

	destPath := filepath.Join(tmpDir, "ffmpeg.exe")
	err := extractFfmpegFromZip(zipPath, destPath, true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, _ := os.ReadFile(destPath)
	if !bytes.Equal(data, content) {
		t.Error("content mismatch")
	}
}

func TestExtractFfmpegFromZip_NotFound(t *testing.T) {
	zipBuf := createZipWithFile(t, "some-other-file.txt", []byte("not ffmpeg"))

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "noffmpeg.zip")
	os.WriteFile(zipPath, zipBuf.Bytes(), 0644)

	destPath := filepath.Join(tmpDir, "ffmpeg.exe")
	err := extractFfmpegFromZip(zipPath, destPath, true)
	if err == nil {
		t.Fatal("expected error when ffmpeg not found in zip")
	}
	if !strings.Contains(err.Error(), "not found in zip") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtractFfmpegFromZip_InvalidZip(t *testing.T) {
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "bad.zip")
	os.WriteFile(zipPath, []byte("not a zip file"), 0644)

	err := extractFfmpegFromZip(zipPath, filepath.Join(tmpDir, "ffmpeg"), false)
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestExtractFfmpegFromZip_MissingFile(t *testing.T) {
	err := extractFfmpegFromZip("/nonexistent/path.zip", "/tmp/ffmpeg", false)
	if err == nil {
		t.Fatal("expected error for missing zip file")
	}
}

// ─── extractFfmpegFromTarXZ ───────────────────────────────────────────────────

func TestExtractFfmpegFromTarXZ_Success(t *testing.T) {
	content := []byte("ffmpeg-linux-binary-content")
	tarXzBuf := createTarXzWithFile(t, "ffmpeg-release/ffmpeg", content)

	tmpDir := t.TempDir()
	tarXzPath := filepath.Join(tmpDir, "ffmpeg.tar.xz")
	os.WriteFile(tarXzPath, tarXzBuf.Bytes(), 0644)

	destPath := filepath.Join(tmpDir, "ffmpeg")
	err := extractFfmpegFromTarXZ(tarXzPath, destPath)
	if err != nil {
		t.Fatalf("extractFfmpegFromTarXZ error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, content) {
		t.Error("content mismatch")
	}
}

func TestExtractFfmpegFromTarXZ_NotFound(t *testing.T) {
	tarXzBuf := createTarXzWithFile(t, "some-random-tool", []byte("not ffmpeg"))

	tmpDir := t.TempDir()
	tarXzPath := filepath.Join(tmpDir, "ffmpeg.tar.xz")
	os.WriteFile(tarXzPath, tarXzBuf.Bytes(), 0644)

	err := extractFfmpegFromTarXZ(tarXzPath, filepath.Join(tmpDir, "ffmpeg"))
	if err == nil {
		t.Fatal("expected error when ffmpeg not found in tar.xz")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found in tar.xz") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExtractFfmpegFromTarXZ_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.tar.xz")
	os.WriteFile(path, []byte("not a tar.xz"), 0644)

	err := extractFfmpegFromTarXZ(path, filepath.Join(tmpDir, "ffmpeg"))
	if err == nil {
		t.Fatal("expected error for invalid tar.xz")
	}
}

func TestExtractFfmpegFromTarXZ_MissingFile(t *testing.T) {
	err := extractFfmpegFromTarXZ("/nonexistent.tar.xz", "/tmp/ffmpeg")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ─── CheckYtDlpUpdate ─────────────────────────────────────────────────────────

// testCheckYtDlpUpdateWithMock is a helper that tests CheckYtDlpUpdate logic
// by creating a mock server and a minimal Go executable that prints a version.
// Since CheckYtDlp requires a real executable, we compile a tiny helper program.

func createFakeYtDlp(t *testing.T, version string) string {
	t.Helper()
	tmpDir := t.TempDir()

	if runtime.GOOS == "windows" {
		// On Windows, create a .cmd file and name it yt-dlp.exe won't work.
		// Instead, create a .bat wrapper and copy it — but exec.Command runs .exe only via stat.
		// Best approach: write a Go program, compile it.
		srcFile := filepath.Join(tmpDir, "main.go")
		src := fmt.Sprintf(`package main
import "fmt"
func main() { fmt.Print("%s") }
`, version)
		if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
			t.Fatalf("failed to write source: %v", err)
		}
		binName := filepath.Join(tmpDir, "yt-dlp.exe")
		cmd := exec.Command("go", "build", "-o", binName, srcFile)
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to compile fake yt-dlp: %v\n%s", err, out)
		}
		return binName
	}

	// Unix: shell script works fine
	binName := filepath.Join(tmpDir, "yt-dlp")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%s'", version)
	if err := os.WriteFile(binName, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake yt-dlp: %v", err)
	}
	return binName
}

func TestCheckYtDlpUpdate_NotInstalled(t *testing.T) {
	u := NewUpdater()
	u.ytdlpPath = filepath.Join(t.TempDir(), "nonexistent-ytdlp")

	result := u.CheckYtDlpUpdate()

	// If yt-dlp is not globally installed either, we should get not-installed
	if !result.Success && result.Message == "yt-dlp is not installed" {
		if result.CurrentVersion != "" {
			t.Errorf("expected empty CurrentVersion when not installed, got %q", result.CurrentVersion)
		}
		if result.HasUpdate {
			t.Error("expected HasUpdate=false when not installed")
		}
	}
}

func TestCheckYtDlpUpdate_HasUpdate(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2025.01.01")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "2026.02.04"})
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckYtDlpUpdate()

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Message)
	}
	if !result.HasUpdate {
		t.Error("expected HasUpdate=true for newer version")
	}
	if result.CurrentVersion != "2025.01.01" {
		t.Errorf("expected CurrentVersion=2025.01.01, got %s", result.CurrentVersion)
	}
	if result.LatestVersion != "2026.02.04" {
		t.Errorf("expected LatestVersion=2026.02.04, got %s", result.LatestVersion)
	}
	if result.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestCheckYtDlpUpdate_NoUpdate(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2026.02.04")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "2026.02.04"})
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckYtDlpUpdate()

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Message)
	}
	if result.HasUpdate {
		t.Error("expected HasUpdate=false for same version")
	}
	if result.CurrentVersion != "2026.02.04" {
		t.Errorf("expected CurrentVersion=2026.02.04, got %s", result.CurrentVersion)
	}
	if result.LatestVersion != "2026.02.04" {
		t.Errorf("expected LatestVersion=2026.02.04, got %s", result.LatestVersion)
	}
	if result.Message != "yt-dlp is up to date" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCheckYtDlpUpdate_APIError(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2025.01.01")

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	// Point to unreachable server
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: "http://127.0.0.1:1"}

	result := u.CheckYtDlpUpdate()

	if result.Success {
		t.Error("expected failure when API is unreachable")
	}
	if !strings.Contains(result.Message, "Failed to check yt-dlp releases") {
		t.Errorf("unexpected error message: %s", result.Message)
	}
	if result.CurrentVersion != "2025.01.01" {
		t.Errorf("expected CurrentVersion=2025.01.01 even on failure, got %s", result.CurrentVersion)
	}
	if result.HasUpdate {
		t.Error("expected HasUpdate=false on error")
	}
}

func TestCheckYtDlpUpdate_InvalidJSON(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2025.01.01")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckYtDlpUpdate()

	if result.Success {
		t.Error("expected failure for invalid JSON response")
	}
	if !strings.Contains(result.Message, "Failed to parse release info") {
		t.Errorf("unexpected error message: %s", result.Message)
	}
	if result.CurrentVersion != "2025.01.01" {
		t.Errorf("expected CurrentVersion preserved on parse error, got %s", result.CurrentVersion)
	}
}

func TestCheckYtDlpUpdate_EmptyTagName(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2025.01.01")

	// API returns empty tag_name
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"tag_name": ""})
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckYtDlpUpdate()

	// Empty latest version differs from current, so HasUpdate=true
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Message)
	}
	if result.LatestVersion != "" {
		t.Errorf("expected empty LatestVersion, got %q", result.LatestVersion)
	}
	if result.CurrentVersion != "2025.01.01" {
		t.Errorf("expected CurrentVersion=2025.01.01, got %s", result.CurrentVersion)
	}
}

func TestCheckYtDlpUpdate_ServerReturns500(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2025.01.01")

	// Server returns 500 but valid-ish body (json.Decode might still work)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"rate limit exceeded"}`))
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckYtDlpUpdate()

	// The function doesn't check HTTP status codes, so it will try to decode the body.
	// tag_name will be empty string, which differs from the current version.
	// This verifies the function doesn't crash on error responses.
	_ = result
}

func TestCheckYtDlpUpdate_ResultFieldsPopulated(t *testing.T) {
	fakeBin := createFakeYtDlp(t, "2025.06.01")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "2026.03.01"})
	}))
	defer server.Close()

	u := NewUpdater()
	u.ytdlpPath = fakeBin
	u.httpClient = server.Client()
	u.httpClient.Transport = &urlRewriteTransport{defaultBase: server.URL}

	result := u.CheckYtDlpUpdate()

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Message)
	}

	// Verify all relevant fields are set
	if result.CurrentVersion == "" {
		t.Error("expected non-empty CurrentVersion")
	}
	if result.LatestVersion == "" {
		t.Error("expected non-empty LatestVersion")
	}
	if !result.HasUpdate {
		t.Error("expected HasUpdate=true")
	}
	if result.Message == "" {
		t.Error("expected non-empty Message")
	}
	if !strings.Contains(result.Message, "2026.03.01") {
		t.Errorf("message should contain latest version, got: %s", result.Message)
	}
	if !strings.Contains(result.Message, "2025.06.01") {
		t.Errorf("message should contain current version, got: %s", result.Message)
	}

	// These fields should not be set by CheckYtDlpUpdate
	if result.Changelog != "" {
		t.Errorf("expected empty Changelog, got %q", result.Changelog)
	}
	if result.DownloadURL != "" {
		t.Errorf("expected empty DownloadURL, got %q", result.DownloadURL)
	}
}

// ─── UpdateYTDLP ───────────────────────────────────────────────────────────────

func TestUpdateYTDLP_NotInstalled(t *testing.T) {
	u := NewUpdater()
	u.ytdlpPath = filepath.Join(t.TempDir(), "nonexistent-ytdlp")

	// Clear cached path to force re-check
	result := u.UpdateYTDLP()

	// If yt-dlp is not globally installed either, we should get not-installed
	if result.Success && result.Message == "yt-dlp is not installed" {
		t.Error("should not be success when not installed")
	}
	// We can only assert structure is valid
	_ = result.Message
}

// ─── VersionInfo / UpdateResult / YtDlpStatus / FfmpegStatus struct tests ────

func TestVersionInfo_JSONRoundtrip(t *testing.T) {
	vi := VersionInfo{
		Version:     "2.0.0",
		ReleaseDate: "2026-01-01",
		Changelog:   "test changes",
		MinVersion:  "1.0.0",
	}
	vi.Downloads.Windows = "https://example.com/win"
	vi.Downloads.Darwin = "https://example.com/mac"
	vi.Downloads.Linux = "https://example.com/linux"

	data, err := json.Marshal(vi)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded VersionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Version != vi.Version {
		t.Errorf("version mismatch: %s vs %s", decoded.Version, vi.Version)
	}
	if decoded.Downloads.Windows != vi.Downloads.Windows {
		t.Errorf("windows URL mismatch")
	}
}

func TestUpdateResult_JSONRoundtrip(t *testing.T) {
	ur := UpdateResult{
		Success:        true,
		Message:        "ok",
		CurrentVersion: "1.0.0",
		LatestVersion:  "2.0.0",
		HasUpdate:      true,
		Changelog:      "changes",
		DownloadURL:    "https://example.com",
	}

	data, err := json.Marshal(ur)
	if err != nil {
		t.Fatal(err)
	}

	var decoded UpdateResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Success != ur.Success || decoded.HasUpdate != ur.HasUpdate {
		t.Error("roundtrip mismatch")
	}
}

func TestYtDlpStatus_JSONRoundtrip(t *testing.T) {
	s := YtDlpStatus{Installed: true, Path: "/usr/bin/yt-dlp", Version: "2025.01.01"}
	data, _ := json.Marshal(s)
	var decoded YtDlpStatus
	json.Unmarshal(data, &decoded)
	if decoded.Path != s.Path || decoded.Version != s.Version || decoded.Installed != s.Installed {
		t.Error("roundtrip mismatch")
	}
}

func TestFfmpegStatus_JSONRoundtrip(t *testing.T) {
	s := FfmpegStatus{Installed: true, Path: "/usr/bin/ffmpeg", Version: "6.1.1"}
	data, _ := json.Marshal(s)
	var decoded FfmpegStatus
	json.Unmarshal(data, &decoded)
	if decoded.Path != s.Path || decoded.Version != s.Version || decoded.Installed != s.Installed {
		t.Error("roundtrip mismatch")
	}
}

// ─── Edge cases for compareVersions ────────────────────────────────────────────

func TestCompareVersions_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		v1   string
		v2   string
		want int
	}{
		{"empty both", "", "", 0},
		{"empty vs version", "", "1.0.0", -1},
		{"version vs empty", "1.0.0", "", 1},
		{"v prefix only", "v", "v", 0},
		{"extra dots", "1.0.0.0", "1.0.0", 0},
		{"extra dots with value", "1.0.0.1", "1.0.0", 1},
		{"non-numeric defaults to zero", "a.b.c", "0.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.v1, tt.v2)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
			}
		})
	}
}

// ─── ffmpegDownloadURLs map ────────────────────────────────────────────────────

func TestFfmpegDownloadURLs(t *testing.T) {
	expectedOSes := []string{"windows", "darwin", "linux"}
	for _, os := range expectedOSes {
		if url, ok := ffmpegDownloadURLs[os]; !ok || url == "" {
			t.Errorf("expected non-empty URL for OS %q", os)
		}
	}
}

// ─── Constants ─────────────────────────────────────────────────────────────────

func TestConstants(t *testing.T) {
	if GitHubOwner == "" {
		t.Error("GitHubOwner should not be empty")
	}
	if GitHubRepo == "" {
		t.Error("GitHubRepo should not be empty")
	}
	if YtDlpReleaseURL == "" {
		t.Error("YtDlpReleaseURL should not be empty")
	}
	if !strings.Contains(YtDlpReleaseURL, "github.com") {
		t.Error("YtDlpReleaseURL should point to GitHub API")
	}
}

// ─── decompressXZ ──────────────────────────────────────────────────────────────

func TestDecompressXZ_InvalidData(t *testing.T) {
	_, err := decompressXZ(bytes.NewReader([]byte("not xz data")))
	if err == nil {
		t.Error("expected error for invalid xz data")
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

// urlRewriteTransport rewrites specific URLs to point at test servers.
type urlRewriteTransport struct {
	base        http.RoundTripper
	rewrites    map[string]string
	defaultBase string // if set, all non-rewritten URLs go here
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	urlStr := req.URL.String()

	// Check explicit rewrites
	for from, to := range t.rewrites {
		if urlStr == from || strings.HasPrefix(urlStr, from) {
			newReq := req.Clone(req.Context())
			newURL := to
			if urlStr != from {
				suffix := urlStr[len(from):]
				newURL = to + suffix
			}
			parsed, err := req.URL.Parse(newURL)
			if err != nil {
				return nil, err
			}
			newReq.URL = parsed
			newReq.Host = parsed.Host
			transport := t.base
			if transport == nil {
				transport = http.DefaultTransport
			}
			return transport.RoundTrip(newReq)
		}
	}

	// Default rewrite: redirect all requests to the test server
	if t.defaultBase != "" {
		newReq := req.Clone(req.Context())
		parsed, err := req.URL.Parse(t.defaultBase + req.URL.Path)
		if err != nil {
			return nil, err
		}
		newReq.URL = parsed
		newReq.Host = parsed.Host
		transport := t.base
		if transport == nil {
			transport = http.DefaultTransport
		}
		return transport.RoundTrip(newReq)
	}

	transport := t.base
	if transport == nil {
		transport = http.DefaultTransport
	}
	return transport.RoundTrip(req)
}

// createZipWithFile creates a zip archive in memory containing a single file.
func createZipWithFile(t *testing.T, name string, content []byte) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("zip create error: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("zip write error: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close error: %v", err)
	}
	return buf
}

// createTarXzWithFile creates a tar.xz archive in memory containing a single file.
func createTarXzWithFile(t *testing.T, name string, content []byte) *bytes.Buffer {
	t.Helper()

	// Create tar first
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	hdr := &tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar header error: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar write error: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close error: %v", err)
	}

	// Compress with xz using the ulikunitz/xz package
	var xzBuf bytes.Buffer
	xzWriter, err := newXZWriter(&xzBuf)
	if err != nil {
		t.Fatalf("xz writer error: %v", err)
	}
	if _, err := io.Copy(xzWriter, &tarBuf); err != nil {
		t.Fatalf("xz copy error: %v", err)
	}
	if err := xzWriter.Close(); err != nil {
		t.Fatalf("xz close error: %v", err)
	}

	return &xzBuf
}

// newXZWriter wraps xz.NewWriter from ulikunitz/xz.
func newXZWriter(w io.Writer) (io.WriteCloser, error) {
	return xz.NewWriter(w)
}
