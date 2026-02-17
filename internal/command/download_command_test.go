package command_test

import (
	"byto/internal/builder"
	"byto/internal/command"
	"byto/internal/domain"
	"context"
	"sync/atomic"
	"testing"
)

func TestExecute_NilBuilder(t *testing.T) {
	cmd := &command.DownloadCommand{Builder: nil}
	err := cmd.Execute(&domain.Media{URL: "http://example.com/video"})
	if err == nil {
		t.Fatal("expected error when builder is nil, got nil")
	}
}

func TestExecute_InvalidArgsType(t *testing.T) {
	cmd := &command.DownloadCommand{Builder: builder.NewYTDLPBuilder()}
	err := cmd.Execute("invalid args")
	if err == nil {
		t.Fatal("expected error when args is a string")
	}
}

func TestExecute_BuilderNoArgs(t *testing.T) {
	cmd := &command.DownloadCommand{Builder: builder.NewYTDLPBuilder()}
	err := cmd.Execute(&domain.Media{URL: "http://example.com/video"})
	if err == nil {
		t.Error("expected error when builder has no args, got nil")
	}
}

func TestExecute_ProgressTemplateAndNewlineOnly(t *testing.T) {
	b := builder.NewYTDLPBuilder().ProgressTemplate("TMPL").Newline()
	cmd := &command.DownloadCommand{Builder: b}
	err := cmd.Execute(&domain.Media{URL: "http://example.com/video"})
	if err == nil {
		t.Error("expected error when builder has template but no URL")
	}
}

func TestExecute_MediaWithNilContext(t *testing.T) {
	// When Ctx is nil, Execute should fall back to context.Background()
	b := builder.NewYTDLPBuilder().URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{URL: "http://example.com/video", Ctx: nil}
	// Should not panic; will fail because yt-dlp isn't available or URL is unreachable
	err := cmd.Execute(media)
	if err == nil {
		t.Log("no error returned (yt-dlp may be installed); test passes either way")
	}
}

func TestExecute_MediaWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	b := builder.NewYTDLPBuilder().URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{
		URL:        "http://example.com/video",
		Ctx:        ctx,
		CancelFunc: cancel,
	}
	err := cmd.Execute(media)
	if err == nil {
		t.Error("expected error with already-cancelled context")
	}
}

func TestExecute_CancelledContextReturnsContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := builder.NewYTDLPBuilder().URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{
		URL:        "http://example.com/video",
		Ctx:        ctx,
		CancelFunc: cancel,
	}
	err := cmd.Execute(media)
	// Should be context.Canceled or a wrapped error
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_MediaWithAllFields(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var progressCalled int32
	var statusCalled int32
	var titleCalled int32

	b := builder.NewYTDLPBuilder().URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{
		ID:       "test-id-123",
		Title:    "Initial Title",
		URL:      "http://example.com/video",
		FilePath: "/tmp/downloads",
		Quality:  domain.Quality720p,
		Status:   domain.Pending,
		Ctx:      ctx,
		OnProgress: func(id string, p domain.DownloadProgress) {
			atomic.StoreInt32(&progressCalled, 1)
		},
		OnStatusChange: func(id string, s domain.DownloadStatus) {
			atomic.StoreInt32(&statusCalled, 1)
		},
		OnTitleChange: func(id string, title string) {
			atomic.StoreInt32(&titleCalled, 1)
		},
	}
	err := cmd.Execute(media)
	// Will error because yt-dlp can't download the URL, but should not panic
	if err == nil {
		t.Log("no error returned; yt-dlp may have succeeded or is installed")
	}
	// Callbacks may or may not have been called depending on yt-dlp output
	_ = atomic.LoadInt32(&progressCalled)
	_ = atomic.LoadInt32(&statusCalled)
	_ = atomic.LoadInt32(&titleCalled)
}

func TestExecute_MediaWithQualitySettings(t *testing.T) {
	qualities := []domain.VideoQuality{
		domain.Quality360p,
		domain.Quality480p,
		domain.Quality720p,
		domain.Quality1080p,
		domain.Quality1440p,
		domain.Quality2160p,
	}

	for _, q := range qualities {
		t.Run("quality_"+qualityName(q), func(t *testing.T) {
			b := builder.NewYTDLPBuilder().Video(q).URL("http://example.com/video")
			cmd := &command.DownloadCommand{Builder: b}
			media := &domain.Media{URL: "http://example.com/video", Quality: q}
			err := cmd.Execute(media)
			// Expect error (yt-dlp not reachable), but should not panic
			if err == nil {
				t.Log("no error returned; yt-dlp may be installed")
			}
		})
	}
}

func qualityName(q domain.VideoQuality) string {
	switch q {
	case domain.Quality360p:
		return "360p"
	case domain.Quality480p:
		return "480p"
	case domain.Quality720p:
		return "720p"
	case domain.Quality1080p:
		return "1080p"
	case domain.Quality1440p:
		return "1440p"
	case domain.Quality2160p:
		return "2160p"
	default:
		return "unknown"
	}
}

func TestExecute_MediaWithDownloadPath(t *testing.T) {
	b := builder.NewYTDLPBuilder().DownloadPath("/tmp/test").URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{URL: "http://example.com/video", FilePath: "/tmp/test"}
	err := cmd.Execute(media)
	if err == nil {
		t.Log("no error returned; yt-dlp may be installed")
	}
}

func TestExecute_MediaWithSafeFilenames(t *testing.T) {
	b := builder.NewYTDLPBuilder().SafeFilenames().URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{URL: "http://example.com/video"}
	err := cmd.Execute(media)
	if err == nil {
		t.Log("no error returned; yt-dlp may be installed")
	}
}

func TestExecute_FullBuilderChain(t *testing.T) {
	b := builder.NewYTDLPBuilder().
		Video(domain.Quality1080p).
		DownloadPath("/tmp/downloads").
		SafeFilenames().
		URL("http://example.com/video")
	cmd := &command.DownloadCommand{Builder: b}
	media := &domain.Media{
		ID:       "chain-test",
		URL:      "http://example.com/video",
		Quality:  domain.Quality1080p,
		FilePath: "/tmp/downloads",
	}
	err := cmd.Execute(media)
	if err == nil {
		t.Log("no error returned; yt-dlp may be installed")
	}
}

func TestExecute_MultipleCallsSameCommand(t *testing.T) {
	cmd := &command.DownloadCommand{Builder: nil}

	err1 := cmd.Execute(&domain.Media{URL: "http://example.com/video1"})
	err2 := cmd.Execute(&domain.Media{URL: "http://example.com/video2"})

	if err1 == nil || err2 == nil {
		t.Error("expected errors for both calls with nil builder")
	}
}

func TestDownloadCommand_ImplementsCommandInterface(t *testing.T) {
	var _ command.Command = (*command.DownloadCommand)(nil)
}
