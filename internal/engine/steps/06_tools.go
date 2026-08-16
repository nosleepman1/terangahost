package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepTools installe Composer globalement, Supervisor pour les workers de queue, Certbot et Logrotate
type StepTools struct{}

func (s *StepTools) ID() string {
	return "06_tools"
}

func (s *StepTools) Title() string {
	return "Installation de Composer, Supervisor (Queues Laravel), Certbot & Logrotate"
}

func (s *StepTools) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	out, err := r.RunSilent(ctx, "composer --version 2>/dev/null && supervisorctl version 2>/dev/null && echo 'READY'")
	if err == nil && strings.Contains(out, "READY") {
		return true, nil
	}
	return false, nil
}

func (s *StepTools) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	commands := []string{
		// 1. Installation de Supervisor et Certbot
		"apt-get install -y supervisor certbot python3-certbot-nginx",
		"systemctl enable supervisor",
		"systemctl start supervisor",

		// 2. Installation de Composer 2 (Officiel)
		"curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer",
		"chmod +x /usr/local/bin/composer",

		// 3. Configuration du dossier racine /var/www
		"mkdir -p /var/www",
		"chown -R deployer:www-data /var/www",
		"chmod -R 775 /var/www",
	}

	for _, cmd := range commands {
		if _, err := r.RunSilent(ctx, cmd); err != nil {
			return fmt.Errorf("erreur installation outils (%s): %w", cmd, err)
		}
	}

	// 4. Configuration stricte de Logrotate pour éviter la saturation du disque par les logs Laravel et Nginx
	logrotateConfig := `/var/log/nginx/*.log /var/www/*/*/storage/logs/*.log {
    daily
    missingok
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 www-data adm
    sharedscripts
    prerotate
        if [ -d /etc/logrotate.d/httpd-prerotate ]; then \
            run-parts /etc/logrotate.d/httpd-prerotate; \
        fi \
    endscript
    postrotate
        invoke-rc.d nginx rotate >/dev/null 2>&1 || true
    endscript
}
`
	if err := r.Upload(ctx, []byte(logrotateConfig), "/etc/logrotate.d/terangahost", 0644); err != nil {
		return fmt.Errorf("impossible d'installer la configuration logrotate: %w", err)
	}

	return nil
}
