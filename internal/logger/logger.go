package logger

import (
	"log/slog"
	"os"
)

func New(verbose bool) *slog.Logger {
	var log *slog.Logger

	level := slog.LevelError
	if verbose {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	log = slog.New(slog.NewTextHandler(os.Stdout, opts))

	return log
}
