package mocks

import (
	"context"
	"io"
	"strings"
	"sync"
)

// MockRunner implémente domain.Runner en mémoire pour les tests unitaires
type MockRunner struct {
	mu            sync.Mutex
	ExecutedCmds  []string
	UploadedFiles map[string][]byte
	CmdOutputs    map[string]string
	CmdErrors     map[string]error
}

func NewMockRunner() *MockRunner {
	return &MockRunner{
		ExecutedCmds:  make([]string, 0),
		UploadedFiles: make(map[string][]byte),
		CmdOutputs:    make(map[string]string),
		CmdErrors:     make(map[string]error),
	}
}

func (m *MockRunner) Execute(ctx context.Context, cmd string, stdout, stderr io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ExecutedCmds = append(m.ExecutedCmds, cmd)

	for pattern, err := range m.CmdErrors {
		if strings.Contains(cmd, pattern) {
			return err
		}
	}

	for pattern, out := range m.CmdOutputs {
		if strings.Contains(cmd, pattern) && stdout != nil {
			_, _ = stdout.Write([]byte(out))
		}
	}

	return nil
}

func (m *MockRunner) RunSilent(ctx context.Context, cmd string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ExecutedCmds = append(m.ExecutedCmds, cmd)

	for pattern, err := range m.CmdErrors {
		if strings.Contains(cmd, pattern) {
			return "", err
		}
	}

	for pattern, out := range m.CmdOutputs {
		if strings.Contains(cmd, pattern) {
			return out, nil
		}
	}

	return "OK", nil
}

func (m *MockRunner) Upload(ctx context.Context, content []byte, remotePath string, mode uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UploadedFiles[remotePath] = content
	return nil
}

func (m *MockRunner) FileExists(ctx context.Context, remotePath string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.UploadedFiles[remotePath]
	return exists, nil
}

func (m *MockRunner) Close() error {
	return nil
}

func (m *MockRunner) HasExecuted(cmdSubstring string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.ExecutedCmds {
		if strings.Contains(c, cmdSubstring) {
			return true
		}
	}
	return false
}
