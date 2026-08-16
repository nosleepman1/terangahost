package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepHandshake valide la connexion, attend la libération du verrou APT (cloud-init), et vérifie l'OS
type StepHandshake struct{}

func (s *StepHandshake) ID() string {
	return "00_handshake"
}

func (s *StepHandshake) Title() string {
	return "Vérification de l'OS et attente de la libération d'APT (cloud-init)"
}

func (s *StepHandshake) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	// Cette étape doit toujours s'exécuter pour garantir que le verrou APT est libre
	return false, nil
}

func (s *StepHandshake) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	// 1. Vérification de la distribution (Ubuntu obligatoire)
	out, err := r.RunSilent(ctx, "cat /etc/os-release")
	if err != nil {
		return fmt.Errorf("impossible de lire /etc/os-release: %w", err)
	}
	if !strings.Contains(strings.ToLower(out), "ubuntu") {
		return domain.ErrUnsupportedOS
	}

	// 2. Boucle active d'attente pour le déverrouillage de /var/lib/dpkg/lock-frontend (cloud-init)
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		checkCmd := "fuser /var/lib/dpkg/lock-frontend /var/lib/apt/lists/lock /var/lib/dpkg/lock >/dev/null 2>&1 && echo 'LOCKED' || echo 'FREE'"
		status, err := r.RunSilent(ctx, checkCmd)
		if err == nil && strings.Contains(status, "FREE") {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
			// Attendre 3 secondes avant de re-tester
		}
	}

	return domain.ErrAptLockTimeout
}
