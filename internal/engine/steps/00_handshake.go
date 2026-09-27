package steps

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// StepHandshake vérifie les privilèges et l'OS, puis attend la fin de cloud-init et la libération d'APT.
type StepHandshake struct {
	Interval time.Duration // délai entre deux tests du verrou APT (3s par défaut)
	Retries  int           // nombre de tests (100 par défaut, soit 5 minutes)
}

func (s *StepHandshake) ID() string { return "00_handshake" }

func (s *StepHandshake) Title() string {
	return "Vérification des privilèges, de l'OS et disponibilité d'APT"
}

func (s *StepHandshake) PreCheck(context.Context, domain.Runner, *domain.Server) (bool, error) {
	return false, nil // toujours exécutée
}

func (s *StepHandshake) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	uid, err := r.RunSilent(ctx, "id -u")
	if err != nil || strings.TrimSpace(uid) != "0" {
		return fmt.Errorf("%w (utilisateur %q)", domain.ErrSudoRequired, srv.AdminUser)
	}

	osRelease, err := r.RunSilent(ctx, "cat /etc/os-release")
	if err != nil {
		return fmt.Errorf("impossible de lire /etc/os-release: %w", err)
	}
	if err := checkUbuntu(osRelease); err != nil {
		return err
	}

	_, _ = r.RunSilent(ctx, "if command -v cloud-init >/dev/null 2>&1; then timeout 900 cloud-init status --wait >/dev/null 2>&1 || true; fi")

	interval, retries := s.Interval, s.Retries
	if interval == 0 {
		interval = 3 * time.Second
	}
	if retries == 0 {
		retries = 100
	}
	check := `if command -v fuser >/dev/null 2>&1; then fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/lib/apt/lists/lock >/dev/null 2>&1 && echo LOCKED || echo FREE; ` +
		`else pgrep -x 'apt|apt-get|dpkg|unattended-upgr' >/dev/null && echo LOCKED || echo FREE; fi`
	for i := 0; i < retries; i++ {
		status, err := r.RunSilent(ctx, check)
		if err == nil && strings.Contains(status, "FREE") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return domain.ErrAptLockTimeout
}

// checkUbuntu accepte Ubuntu 22.04 ou plus récent.
func checkUbuntu(osRelease string) error {
	fields := map[string]string{}
	for _, line := range strings.Split(osRelease, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			fields[k] = strings.Trim(v, `"'`)
		}
	}
	if fields["ID"] != "ubuntu" {
		return fmt.Errorf("%w: %s détecté", domain.ErrUnsupportedOS, fields["PRETTY_NAME"])
	}
	major, minor, _ := strings.Cut(fields["VERSION_ID"], ".")
	ma, err1 := strconv.Atoi(major)
	mi, err2 := strconv.Atoi(minor)
	if err1 != nil || err2 != nil || ma < 22 || (ma == 22 && mi < 4) {
		return fmt.Errorf("%w: Ubuntu %s détecté", domain.ErrUnsupportedOS, fields["VERSION_ID"])
	}
	return nil
}
