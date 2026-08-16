package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepSecurity installe les outils essentiels, crée l'utilisateur 'deployer', configure UFW et Fail2ban
type StepSecurity struct{}

func (s *StepSecurity) ID() string {
	return "02_security"
}

func (s *StepSecurity) Title() string {
	return "Sécurisation du VPS (Création user 'deployer', Pare-feu UFW, Fail2ban)"
}

func (s *StepSecurity) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	out, err := r.RunSilent(ctx, "id -u deployer 2>/dev/null && ufw status | grep -q 'Status: active' && echo 'READY'")
	if err == nil && strings.Contains(out, "READY") {
		return true, nil
	}
	return false, nil
}

func (s *StepSecurity) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	commands := []string{
		// 1. Mise à jour de base & paquets requis
		"apt-get update -y",
		"apt-get install -y ufw fail2ban curl git unzip zip software-properties-common apt-transport-https ca-certificates gnupg lsb-release htop",

		// 2. Création de l'utilisateur deployer et configuration de son home
		"id -u deployer >/dev/null 2>&1 || useradd -m -s /bin/bash -g www-data deployer",
		"mkdir -p /home/deployer/.ssh",
		"chmod 700 /home/deployer/.ssh",

		// 3. Copie des clés SSH autorisées de root vers deployer
		"[ -f /root/.ssh/authorized_keys ] && cp /root/.ssh/authorized_keys /home/deployer/.ssh/authorized_keys && chmod 600 /home/deployer/.ssh/authorized_keys && chown -R deployer:www-data /home/deployer/.ssh || true",

		// 4. Configuration du Pare-feu UFW (Ports 22, 80, 443 uniquement)
		"ufw default deny incoming",
		"ufw default allow outgoing",
		fmt.Sprintf("ufw allow %d/tcp comment 'SSH'", srv.SSHPort),
		"ufw allow 80/tcp comment 'HTTP'",
		"ufw allow 443/tcp comment 'HTTPS'",
		"echo 'y' | ufw enable",

		// 5. Activation de Fail2ban
		"systemctl enable fail2ban",
		"systemctl restart fail2ban",
	}

	for _, cmd := range commands {
		if _, err := r.RunSilent(ctx, cmd); err != nil {
			return fmt.Errorf("erreur sécurisation (%s): %w", cmd, err)
		}
	}

	return nil
}
