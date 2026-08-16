package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepSwap configure 2 Go de swap avec swappiness=10 pour immuniser le VPS contre l'OOM Killer
type StepSwap struct{}

func (s *StepSwap) ID() string {
	return "01_swap"
}

func (s *StepSwap) Title() string {
	return "Configuration de la mémoire SWAP (2 Go) et protection OOM Killer"
}

func (s *StepSwap) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	out, err := r.RunSilent(ctx, "swapon --show")
	if err == nil && strings.Contains(out, "/swapfile") {
		return true, nil
	}
	return false, nil
}

func (s *StepSwap) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	commands := []string{
		"fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048",
		"chmod 600 /swapfile",
		"mkswap /swapfile",
		"swapon /swapfile",
		"grep -q '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab",
		"sysctl vm.swappiness=10",
		"sysctl vm.vfs_cache_pressure=50",
		"grep -q 'vm.swappiness' /etc/sysctl.conf || echo 'vm.swappiness=10' >> /etc/sysctl.conf",
		"grep -q 'vm.vfs_cache_pressure' /etc/sysctl.conf || echo 'vm.vfs_cache_pressure=50' >> /etc/sysctl.conf",
	}

	for _, cmd := range commands {
		if _, err := r.RunSilent(ctx, cmd); err != nil {
			return fmt.Errorf("erreur configuration swap (%s): %w", cmd, err)
		}
	}

	return nil
}
