package builder

import (
	"byto/internal/domain"
)

type YTDLPBuilder struct {
	args []string
}

func NewYTDLPBuilder() *YTDLPBuilder { return &YTDLPBuilder{} }

// "[byto:title] %(info.title)s [byto:downloaded_bytes] %(progress.downloaded_bytes)d [byto:total_bytes] %(progress.total_bytes)d"
func (y *YTDLPBuilder) ProgressTemplate(template string) *YTDLPBuilder {
	y.args = append(y.args, "--progress-template", template)
	return y
}

func (y *YTDLPBuilder) Quality(quality domain.VideoQuality) *YTDLPBuilder {
	switch quality {
	case domain.Quality360p:
		y.args = append(y.args, "-f", "best[height<=360]")
	case domain.Quality480p:
		y.args = append(y.args, "-f", "best[height<=480]")
	case domain.Quality720p:
		y.args = append(y.args, "-f", "best[height<=720]")
	case domain.Quality1080p:
		y.args = append(y.args, "-f", "best[height<=1080]")
	case domain.Quality1440p:
		y.args = append(y.args, "-f", "best[height<=1440]")
	case domain.Quality2160p:
		y.args = append(y.args, "-f", "best[height<=2160]")
	default:
		y.args = append(y.args, "-f", "best")
	}
	return y
}

func (y *YTDLPBuilder) DownloadPath(path string) *YTDLPBuilder {
	y.args = append(y.args, "-o", path+"/%(title)s.%(ext)s")
	return y
}

func (y *YTDLPBuilder) URL(url string) *YTDLPBuilder {
	y.args = append(y.args, url)
	return y
}

func (y *YTDLPBuilder) Update() *YTDLPBuilder {
	y.args = append(y.args, "--update")
	return y
}

func (y *YTDLPBuilder) Build() []string {
	return y.args
}
