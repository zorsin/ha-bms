package logger

import (
	"log/slog"
	"os"
)

func Get(level ...string) *slog.Logger {
	logLevel := slog.LevelInfo
	if len(level) > 0 {
		_ = logLevel.UnmarshalText([]byte(level[0]))
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
}
