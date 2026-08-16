package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// FileLogger gère l'écriture de logs structurés détaillés sur disque pour le debug
type FileLogger struct {
	file   *os.File
	Logger *slog.Logger
}

// NewFileLogger initialise un fichier de log horodaté dans ~/.terangahost/logs/
func NewFileLogger(prefix string) (*FileLogger, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}

	logDir := filepath.Join(home, ".terangahost", "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, "", err
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.log", prefix, timestamp)
	fullPath := filepath.Join(logDir, filename)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, "", err
	}

	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	return &FileLogger{
		file:   f,
		Logger: slog.New(handler),
	}, fullPath, nil
}

// Writer renvoie un io.Writer vers le fichier de log pour y capturer stdout/stderr distants
func (fl *FileLogger) Writer() io.Writer {
	return fl.file
}

// Close ferme le fichier
func (fl *FileLogger) Close() error {
	if fl.file != nil {
		return fl.file.Close()
	}
	return nil
}
