package command

import (
	"bufio"
	"byto/internal/builder"
	"byto/internal/domain"
	"byto/internal/parser"
	"fmt"
	"log"
	"os/exec"
	"strconv"
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

	c.Builder.ProgressTemplate("[byto:title] %(info.title)s [byto:downloaded_bytes] %(progress.downloaded_bytes)d [byto:total_bytes] %(progress.total_bytes)d")
	log.Printf("DownloadCommand: Configured YTDLP builder progress template.")

	ucmd := c.Builder.Build()
	cmd := exec.Command("yt-dlp", ucmd...)
	log.Printf("DownloadCommand: Executing command: yt-dlp %v", ucmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("DownloadCommand: Failed to get stdout pipe: %v", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		log.Printf("DownloadCommand: Failed to start yt-dlp command: %v", err)
		return err
	}
	log.Printf("DownloadCommand: yt-dlp command started successfully.")

	parser := parser.YTDLPDownloadParser{}
	scanner := bufio.NewScanner(stdout)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("YTDLP Output: %s", line) // Log to internal logger
			media.AppendLog(line)                // Append to media's log for UI/user

			parsedData, err := parser.Parse(line)
			if err == nil {
				downloaded, _ := strconv.ParseInt(parsedData["downloaded_bytes"], 10, 64)
				total, _ := strconv.ParseInt(parsedData["total_bytes"], 10, 64)

				percentage := 0
				if total > 0 {
					percentage = int((float64(downloaded) / float64(total)) * 100)
				}
				media.UpdateProgress(downloaded, total, percentage)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("DownloadCommand: Error reading stdout: %v", err)
		}
		log.Printf("DownloadCommand: Finished scanning stdout for media: %s", media.URL)
	}()

	if err := cmd.Wait(); err != nil {
		media.SetStatus(domain.Failed)
		log.Printf("DownloadCommand: yt-dlp command failed for media %s: %v", media.URL, err)
		return err
	}

	media.SetStatus(domain.Completed)
	log.Printf("DownloadCommand: yt-dlp command completed successfully for media: %s", media.URL)
	return nil
}
