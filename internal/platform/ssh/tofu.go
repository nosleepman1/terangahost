package ssh

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/platform/storage"
	"golang.org/x/crypto/ssh"
)

// TOFUCallback implémente le Trust-On-First-Use pour les clés d'hôte SSH : la première empreinte
// vue pour un hôte est mémorisée dans ~/.terangahost/known_hosts, toute modification est refusée.
type TOFUCallback struct {
	knownHostsPath string
	mu             sync.Mutex
}

// NewTOFUCallback crée le validateur d'empreinte.
func NewTOFUCallback() (*TOFUCallback, error) {
	path, err := KnownHostsPath()
	if err != nil {
		return nil, err
	}
	return &TOFUCallback{knownHostsPath: path}, nil
}

// KnownHostsPath retourne le chemin du fichier known_hosts de TerangaHost.
func KnownHostsPath() (string, error) {
	dir, err := storage.DefaultDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("impossible de créer le dossier %s: %w", dir, err)
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// Callback implémente ssh.HostKeyCallback.
func (t *TOFUCallback) Callback() ssh.HostKeyCallback {
	return func(hostname string, _ net.Addr, key ssh.PublicKey) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		sum := sha256.Sum256(key.Marshal())
		fingerprint := hex.EncodeToString(sum[:])

		data, err := os.ReadFile(t.knownHostsPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("impossible de lire %s: %w", t.knownHostsPath, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			parts := strings.Fields(line)
			if len(parts) < 3 || parts[0] != hostname || parts[1] != key.Type() {
				continue
			}
			if parts[2] == fingerprint {
				return nil
			}
			return fmt.Errorf("%w: %s (%s). Si le serveur a été réinstallé, lancez 'terangahost server forget-host --host %s'",
				domain.ErrHostKeyMismatch, hostname, key.Type(), hostname)
		}

		f, err := os.OpenFile(t.knownHostsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = fmt.Fprintf(f, "%s %s %s\n", hostname, key.Type(), fingerprint)
		return err
	}
}

// ForgetHost supprime les empreintes mémorisées pour un hôte ("ip:port").
func ForgetHost(hostport string) (int, error) {
	path, err := KnownHostsPath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var kept []string
	removed := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		if f := strings.Fields(line); len(f) > 0 && f[0] == hostport {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n")
	if out != "" {
		out += "\n"
	}
	return removed, os.WriteFile(path, []byte(out), 0o600)
}
