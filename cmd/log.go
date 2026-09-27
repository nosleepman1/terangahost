package cmd

import (
	"log/slog"

	"github.com/nosleepman1/terangahost/internal/platform/logger"
)

func loggerOf(l *logger.FileLogger) *slog.Logger {
	if l == nil {
		return nil
	}
	return l.Logger
}
