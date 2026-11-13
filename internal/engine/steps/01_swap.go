package steps

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// StepSwap configure 2 Go de swap (swappiness=10) pour protéger le VPS de l'OOM killer,
// notamment pendant les installations Composer sur les petites instances.
type StepSwap struct{}

func (s *StepSwap) ID() string    { return "01_swap" }
func (s *StepSwap) Title() string { return "Mémoire SWAP (2 Go) et protection contre l'OOM killer" }

const sysctlConf = "# Managed by TerangaHost\nvm.swappiness=10\nvm.vfs_cache_pressure=50\n"

func (s *StepSwap) PreCheck(ctx context.Context, r domain.Runner, _ *domain.Server) (bool, error) {
	out, err := r.RunSilent(ctx, "swapon --show=SIZE --bytes --noheadings 2>/dev/null | awk '{s+=$1} END {print s+0}'")
	if err != nil {
		return false, nil
	}
	total, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return total >= 1<<30 && hasContent(ctx, r, "/etc/sysctl.d/99-terangahost.conf", []byte(sysctlConf)), nil
}

func (s *StepSwap) Execute(ctx context.Context, r domain.Runner, _ *domain.Server) error {
	out, _ := r.RunSilent(ctx, "swapon --show=SIZE --bytes --noheadings 2>/dev/null | awk '{s+=$1} END {print s+0}'")
	total, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)

	if total < 1<<30 {
		avail, err := r.RunSilent(ctx, "df -BM --output=avail / | tail -n 1 | tr -dc 0-9")
		if err != nil {
			return fmt.Errorf("lecture de l'espace disque: %w", err)
		}
		if mb, _ := strconv.Atoi(avail); mb < 3072 {
			return fmt.Errorf("espace disque insuffisant pour créer 2 Go de swap (%d Mo libres)", mb)
		}
		if err := runAll(ctx, r, "création du swap",
			"[ -f /swapfile ] || fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048 status=none",
			"chmod 600 /swapfile",
			"swapon --show=NAME --noheadings | grep -qx /swapfile || { mkswap /swapfile >/dev/null && swapon /swapfile; }",
			"grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab",
		); err != nil {
			return err
		}
	}

	if err := r.Upload(ctx, []byte(sysctlConf), "/etc/sysctl.d/99-terangahost.conf", 0o644); err != nil {
		return err
	}
	return runAll(ctx, r, "paramètres noyau", "sysctl -q -p /etc/sysctl.d/99-terangahost.conf")
}
