package parser

import (
	"errors"
	"regexp"
	"strings"
)

type YTDLPDownloadParser struct{}

func (p YTDLPDownloadParser) Parse(input string) (map[string]string, error) {
	// Format: [byto] <title> [downloaded] <bytes> [total] <bytes|NA> [frag] <index|NA> [frags] <count|NA>
	// Title is captured between [byto] and [downloaded] markers
	var logRegex = regexp.MustCompile(`\[byto\]\s+(.+?)\s+\[downloaded\]\s+(\d+|NA)\s+\[total\]\s+(\d+|NA)\s+\[frag\]\s+(\d+|NA)\s+\[frags\]\s+(\d+|NA)$`)

	input = strings.TrimSpace(input)
	matches := logRegex.FindStringSubmatch(input)
	if len(matches) < 6 {
		return nil, errors.New("failed to parse log line: format mismatch")
	}

	result := make(map[string]string)
	result["title"] = strings.TrimSpace(matches[1])
	result["downloaded_bytes"] = matches[2]
	result["total_bytes"] = matches[3]
	result["fragment_index"] = matches[4]
	result["fragment_count"] = matches[5]

	return result, nil
}
