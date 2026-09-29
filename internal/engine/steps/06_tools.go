package steps

import (
	"context"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/templates"
)

// StepTools installe Composer (signature vérifiée), Supervisor, Certbot et la rotation des logs.
type StepTools struct{}

func (s *StepTools) ID() string    { return "06_tools" }
func (s *StepTools) Title() string { return "Composer, Supervisor, Certbot et rotation des logs" }

const renewHookPath = "/etc/letsencrypt/renewal-hooks/deploy/terangahost-reload-nginx"
const renewHook = "#!/bin/sh\n# Managed by TerangaHost : recharge Nginx après chaque renouvellement de certificat\nsystemctl reload nginx\n"

// Installation officielle de Composer avec vérification de la signature SHA-384 de l'installeur.
// Composer est toujours installé dans /usr/local/bin, chemin utilisé par le script de déploiement.
const composerInstall = `if [ ! -x /usr/local/bin/composer ]; then
  set -e
  EXPECTED=$(curl -fsSL https://composer.github.io/installer.sig)
  curl -fsSL https://getcomposer.org/installer -o /tmp/composer-setup.php
  ACTUAL=$(php -r "echo hash_file('sha384', '/tmp/composer-setup.php');")
  if [ "$EXPECTED" != "$ACTUAL" ]; then rm -f /tmp/composer-setup.php; echo "Signature de l'installeur Composer invalide" >&2; exit 1; fi
  php /tmp/composer-setup.php --quiet --install-dir=/usr/local/bin --filename=composer
  rm -f /tmp/composer-setup.php
fi`

func (s *StepTools) PreCheck(ctx context.Context, r domain.Runner, _ *domain.Server) (bool, error) {
	lr, err := templates.Static("logrotate/terangahost.conf")
	if err != nil {
		return false, err
	}
	return packagesInstalled(ctx, r, "supervisor", "certbot") &&
		ready(ctx, r, "[ -x /usr/local/bin/composer ] && [ -x "+renewHookPath+" ]") &&
		hasContent(ctx, r, "/etc/logrotate.d/terangahost", lr), nil
}

func (s *StepTools) Execute(ctx context.Context, r domain.Runner, _ *domain.Server) error {
	if err := runAll(ctx, r, "installation des outils",
		aptInstall("supervisor", "certbot"),
		"systemctl enable --now supervisor",
		composerInstall,
		// /var/www appartient à root ; seuls les dossiers des sites appartiennent au deployer.
		"install -d -m 755 -o root -g root /var/www",
		"install -d -m 755 /var/www/letsencrypt",
	); err != nil {
		return err
	}

	lr, err := templates.Static("logrotate/terangahost.conf")
	if err != nil {
		return err
	}
	if err := r.Upload(ctx, lr, "/etc/logrotate.d/terangahost", 0o644); err != nil {
		return err
	}
	return r.Upload(ctx, []byte(renewHook), renewHookPath, 0o755)
}
