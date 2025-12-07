package domain

import "os"

type Setting struct {
	Quality           VideoQuality `json:"quality"`
	ParallelDownloads int          `json:"parallel_downloads"`
	DownloadPath      string       `json:"download_path"`
}

func NewSetting() *Setting {
	s := &Setting{}

	s.ParallelDownloads = 1
	s.Quality = Quality1080p

	home, err := os.UserHomeDir()
	if err != nil {
		s.DownloadPath = "./downloads"
	} else {
		s.DownloadPath = home + string(os.PathSeparator) + "Downloads"
	}
	return s
}

func (s *Setting) Update(quality VideoQuality, parallelDownloads int, downloadPath string) {
	s.Quality = quality
	s.ParallelDownloads = parallelDownloads
	s.DownloadPath = downloadPath
}
