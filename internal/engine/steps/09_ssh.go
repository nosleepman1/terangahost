package steps

import (
	"context"
	"fmt"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/templates"
)

const sshdHardeningPath = "/etc/ssh/sshd_config.d/00-terangahost.conf"

// StepSSHHardening désactive l'authentification SSH par mot de passe et la connexion root par
// mot de passe. Elle n'est activée que si la connexion courante utilise une clé, pour ne jamais
// verrouiller l'accès au serveur.
type StepSSHHardening struct {
	Enabled bool
}

func (s *StepSSHHardening) ID() string    { return "09_ssh_hardening" }
func (s *StepSSHHardening) Title() string { return "Durcissement SSH (clés uniquement)" }

func (s *StepSSHHardening) PreCheck(ctx context.Context, r domain.Runner, _ *domain.Server) (bool, error) {
	if !s.Enabled {
		return true, nil
	}
	want, err := templates.Static("ssh/sshd_hardening.conf.tmpl")
	if err != nil {
		return false, err
	}
	return hasContent(ctx, r, sshdHardeningPath, want), nil
}

func (s *StepSSHHardening) Execute(ctx context.Context, r domain.Runner, _ *domain.Server) error {
	if !s.Enabled {
		return nil
	}
	if !ready(ctx, r, `grep -Eqi '^[[:space:]]*Include[[:space:]]+/etc/ssh/sshd_config\.d/' /etc/ssh/sshd_config`) {
		return fmt.Errorf("/etc/ssh/sshd_config n'inclut pas sshd_config.d : durcissement impossible")
	}
	conf, err := templates.Static("ssh/sshd_hardening.conf.tmpl")
	if err != nil {
		return err
	}
	if err := r.Upload(ctx, conf, sshdHardeningPath, 0o644); err != nil {
		return err
	}
	if _, err := r.RunSilent(ctx, "mkdir -p /run/sshd && sshd -t"); err != nil {
		_, _ = r.RunSilent(ctx, "rm -f "+sshdHardeningPath)
		return fmt.Errorf("configuration sshd invalide, modification annulée: %w", err)
	}
	return runAll(ctx, r, "rechargement de sshd",
		"systemctl try-reload-or-restart ssh.service 2>/dev/null || systemctl try-reload-or-restart sshd.service")
}
