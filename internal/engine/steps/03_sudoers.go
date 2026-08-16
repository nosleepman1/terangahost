package steps

import (
	"context"
	"fmt"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepSudoers configure des droits sudo restreints et sécurisés pour l'utilisateur 'deployer'
// Lui permettant de recharger Nginx, PHP-FPM, Supervisor sans mot de passe tout en bloquant l'accès root global.
type StepSudoers struct{}

func (s *StepSudoers) ID() string {
	return "03_sudoers"
}

func (s *StepSudoers) Title() string {
	return "Configuration des privilèges sudo restreints (Principe du moindre privilège)"
}

func (s *StepSudoers) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	return r.FileExists(ctx, "/etc/sudoers.d/terangahost-deployer")
}

func (s *StepSudoers) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	sudoersContent := `# Droits restreints TerangaHost pour l'utilisateur deployer
deployer ALL=(ALL) NOPASSWD: /usr/sbin/service nginx *, /usr/sbin/service php*-fpm *, /usr/bin/supervisorctl *, /usr/bin/certbot *
`
	err := r.Upload(ctx, []byte(sudoersContent), "/etc/sudoers.d/terangahost-deployer", 0440)
	if err != nil {
		return fmt.Errorf("impossible de créer /etc/sudoers.d/terangahost-deployer: %w", err)
	}

	// Valider la syntaxe du fichier sudoers avec visudo
	_, err = r.RunSilent(ctx, "visudo -c -f /etc/sudoers.d/terangahost-deployer")
	if err != nil {
		_, _ = r.RunSilent(ctx, "rm -f /etc/sudoers.d/terangahost-deployer")
		return fmt.Errorf("syntaxe sudoers invalide: %w", err)
	}

	return nil
}
