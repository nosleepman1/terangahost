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

	"github.com/teranga-host/terangahost/internal/domain"
	"golang.org/x/crypto/ssh"
)

// TOFUCallback gère le mécanisme Trust-On-First-Use pour la validation des clés d'hôtes SSH.
// Il évite le risque de Man-In-The-Middle sans bloquer inutilement les nouveaux VPS.
type TOFUCallback struct {
	knownHostsPath string
	mu             sync.Mutex
}

// NewTOFUCallback crée un validateur d'empreinte d'hôte basé sur ~/.terangahost/known_hosts
func NewTOFUCallback() (*TOFUCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("impossible de récupérer le dossier utilisateur: %w", err)
	}

	dir := filepath.Join(home, ".terangahost")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("impossible de créer le dossier ~/.terangahost: %w", err)
	}

	return &TOFUCallback{
		knownHostsPath: filepath.Join(dir, "known_hosts"),
	}, nil
}

// Callback implémente ssh.HostKeyCallback
func (t *TOFUCallback) Callback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		t.mu.Lock()
		defer t.mu.Unlock()

		fingerprint := sha256.Sum256(key.Marshal())
		fingerprintHex := hex.EncodeToString(fingerprint[:])
		hostEntry := fmt.Sprintf("%s %s %s\n", hostname, key.Type(), fingerprintHex)

		// Si le fichier n'existe pas encore, on le crée et on fait confiance (First Use)
		if _, err := os.Stat(t.knownHostsPath); os.IsNotExist(err) {
			return os.WriteFile(t.knownHostsPath, []byte(hostEntry), 0600)
		}

		data, err := os.ReadFile(t.knownHostsPath)
		if err != nil {
			return fmt.Errorf("impossible de lire le fichier known_hosts: %w", err)
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 3 && parts[0] == hostname {
				if parts[2] == fingerprintHex {
					// Empreinte validée avec succès
					return nil
				}
				// Alerte : L'hôte a changé de clé sans réinitialisation explicite !
				return fmt.Errorf("%w: l'empreinte pour %s a changé (%s vs %s)", domain.ErrHostKeyMismatch, hostname, parts[2], fingerprintHex)
			}
		}

		// Première fois que cet hôte précis est rencontré : on l'ajoute
		f, err := os.OpenFile(t.knownHostsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.WriteString(hostEntry)
		return err
	}
}
