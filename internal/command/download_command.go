package command

import (
	"bufio"
	"byto/internal/builder"
	"byto/internal/domain"
	"byto/internal/parser"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

type DownloadCommand struct {
	Builder *builder.YTDLPBuilder
}

func (c *DownloadCommand) Execute(args any) error {
	log.Printf("DownloadCommand: Starting execution with args: %+v", args)

	media, ok := args.(*domain.Media)
	if !ok {
		err := fmt.Errorf("invalid arguments, expected *domain.Media")
		log.Printf("DownloadCommand: Argument validation failed: %v", err)
		return err
	}
	log.Printf("DownloadCommand: Processing media: %s", media.URL)

	c.Builder.ProgressTemplate("[byto] %(info.title)s [downloaded] %(progress.downloaded_bytes)s [total] %(progress.total_bytes)s [frag] %(progress.fragment_index)s [frags] %(progress.fragment_count)s")
	c.Builder.Newline() // Force newline after each progress update
	log.Printf("DownloadCommand: Configured YTDLP builder progress template.")

	ucmd := c.Builder.Build()
	ytdlpPath := c.Builder.GetYtDlpPath()

	// Use context for cancellation support
	ctx := media.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, ytdlpPath, ucmd...)
	HideWindow(cmd) // Hide console window on Windows
	log.Printf("DownloadCommand: Executing command: %s %v", ytdlpPath, ucmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("DownloadCommand: Failed to get stdout pipe: %v", err)
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("DownloadCommand: Failed to get stderr pipe: %v", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		log.Printf("DownloadCommand: Failed to start yt-dlp command: %v", err)
		return err
	}
	log.Printf("DownloadCommand: yt-dlp command started successfully.")

	p := parser.YTDLPDownloadParser{}

	processOutput := func(reader io.Reader, name string) {
		scanner := bufio.NewScanner(reader)

		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := ensureUTF8(scanner.Text())
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			log.Printf("YTDLP %s: %s", name, line)
			media.AppendLog(line)

			parsedData, err := p.Parse(line)
			if err == nil {
				// Update title if available
				if title, ok := parsedData["title"]; ok && title != "" && title != "NA" && title != media.Title {
					media.SetTitle(title)
				}

				downloaded, _ := strconv.ParseInt(parsedData["downloaded_bytes"], 10, 64)

				// Handle NA for total_bytes
				totalStr := parsedData["total_bytes"]
				var total int64 = 0
				if totalStr != "NA" && totalStr != "" {
					total, _ = strconv.ParseInt(totalStr, 10, 64)
				}

				// byte-based, fallback to fragment-based
				percentage := 0
				if total > 0 {
					percentage = int((float64(downloaded) / float64(total)) * 100)
				} else {
					// For HLS/fragmented downloads, use fragment progress
					fragIndexStr := parsedData["fragment_index"]
					fragCountStr := parsedData["fragment_count"]
					if fragIndexStr != "NA" && fragCountStr != "NA" && fragIndexStr != "" && fragCountStr != "" {
						fragIndex, _ := strconv.ParseInt(fragIndexStr, 10, 64)
						fragCount, _ := strconv.ParseInt(fragCountStr, 10, 64)
						if fragCount > 0 {
							percentage = int((float64(fragIndex) / float64(fragCount)) * 100)
						}
					}
				}
				media.UpdateProgress(downloaded, total, percentage)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("DownloadCommand: Error reading %s: %v", name, err)
		}
	}

	// Read stdout and stderr concurrently
	go processOutput(stdout, "stdout")
	go processOutput(stderr, "stderr")

	if err := cmd.Wait(); err != nil {
		// Check if the error is due to context cancellation (pause)
		if ctx.Err() == context.Canceled {
			log.Printf("DownloadCommand: Download paused for media: %s", media.URL)
			return context.Canceled
		}
		media.SetStatus(domain.Failed)
		log.Printf("DownloadCommand: yt-dlp command failed for media %s: %v", media.URL, err)
		return err
	}

	media.SetStatus(domain.Completed)
	log.Printf("DownloadCommand: yt-dlp command completed successfully for media: %s", media.URL)
	return nil
}
