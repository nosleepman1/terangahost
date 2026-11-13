// Package logger écrit un journal détaillé de chaque exécution dans ~/.terangahost/logs/.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/nosleepman1/terangahost/internal/platform/storage"
)

// FileLogger gère le fichier journal d'une commande.
type FileLogger struct {
	file   *os.File
	Logger *slog.Logger
	Path   string
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// NewFileLogger crée un fichier journal horodaté.
func NewFileLogger(prefix string) (*FileLogger, error) {
	base, err := storage.DefaultDir()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(base, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s_%s.log", unsafeChars.ReplaceAllString(prefix, "_"), time.Now().Format("2006-01-02_15-04-05"))
	fullPath := filepath.Join(logDir, name)
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &FileLogger{
		file:   f,
		Logger: slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Path:   fullPath,
	}, nil
}

// Writer renvoie un io.Writer vers le fichier (sortie brute des commandes distantes).
func (fl *FileLogger) Writer() io.Writer {
	if fl == nil {
		return nil
	}
	return fl.file
}

// Close ferme le fichier.
func (fl *FileLogger) Close() error {
	if fl != nil && fl.file != nil {
		return fl.file.Close()
	}
	return nil
}
