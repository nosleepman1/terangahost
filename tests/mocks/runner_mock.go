// Package mocks fournit un faux domain.Runner en mémoire pour les tests.
package mocks

import (
	"context"
	"io"
	"strings"
	"sync"
)

type rule struct {
	substr string
	out    string
	err    error
}

// MockRunner enregistre les commandes et répond selon des règles « sous-chaîne → sortie/erreur ».
// Les règles sont évaluées de la plus récente à la plus ancienne ; sans règle, la sortie est vide.
type MockRunner struct {
	mu       sync.Mutex
	Commands []string
	Inputs   map[string][]byte
	Files    map[string][]byte
	Modes    map[string]uint32
	Existing map[string]bool
	rules    []rule
}

// NewMockRunner crée un runner vide.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Inputs:   map[string][]byte{},
		Files:    map[string][]byte{},
		Modes:    map[string]uint32{},
		Existing: map[string]bool{},
	}
}

// On définit la sortie des commandes contenant substr.
func (m *MockRunner) On(substr, out string) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule{substr: substr, out: out})
	return m
}

// Fail fait échouer les commandes contenant substr.
func (m *MockRunner) Fail(substr string, err error) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule{substr: substr, err: err})
	return m
}

func (m *MockRunner) match(cmd string) (string, error) {
	for i := len(m.rules) - 1; i >= 0; i-- {
		if strings.Contains(cmd, m.rules[i].substr) {
			return m.rules[i].out, m.rules[i].err
		}
	}
	return "", nil
}

func (m *MockRunner) Execute(_ context.Context, cmd string, stdout, _ io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Commands = append(m.Commands, cmd)
	out, err := m.match(cmd)
	if out != "" && stdout != nil {
		_, _ = stdout.Write([]byte(out))
	}
	return err
}

func (m *MockRunner) RunSilent(_ context.Context, cmd string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Commands = append(m.Commands, cmd)
	return m.match(cmd)
}

func (m *MockRunner) RunWithInput(_ context.Context, cmd string, input []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Commands = append(m.Commands, cmd)
	m.Inputs[cmd] = append(m.Inputs[cmd], input...)
	return m.match(cmd)
}

func (m *MockRunner) Upload(_ context.Context, content []byte, remotePath string, mode uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Commands = append(m.Commands, "upload "+remotePath)
	if _, err := m.match("upload " + remotePath); err != nil {
		return err
	}
	m.Files[remotePath] = content
	m.Modes[remotePath] = mode
	return nil
}

func (m *MockRunner) FileExists(_ context.Context, remotePath string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, uploaded := m.Files[remotePath]
	return uploaded || m.Existing[remotePath], nil
}

func (m *MockRunner) Close() error { return nil }

// HasExecuted indique si une commande contenant substr a été exécutée.
func (m *MockRunner) HasExecuted(substr string) bool {
	return m.Count(substr) > 0
}

// Count compte les commandes contenant substr.
func (m *MockRunner) Count(substr string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.Commands {
		if strings.Contains(c, substr) {
			n++
		}
	}
	return n
}

// InputFor retourne les données envoyées sur stdin aux commandes contenant substr.
func (m *MockRunner) InputFor(substr string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var b strings.Builder
	for cmd, in := range m.Inputs {
		if strings.Contains(cmd, substr) {
			b.Write(in)
		}
	}
	return b.String()
}
