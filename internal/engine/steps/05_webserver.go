package steps

import (
	"context"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/templates"
)

// StepWebServer installe Nginx avec une configuration globale optimisée pour les API Laravel
// et un serveur par défaut qui refuse les domaines inconnus.
type StepWebServer struct{}

func (s *StepWebServer) ID() string    { return "05_webserver" }
func (s *StepWebServer) Title() string { return "Nginx (FastCGI, TLS, Gzip) et serveur par défaut" }

func (s *StepWebServer) PreCheck(ctx context.Context, r domain.Runner, _ *domain.Server) (bool, error) {
	conf, err := templates.Static("nginx/nginx.conf")
	if err != nil {
		return false, err
	}
	return packagesInstalled(ctx, r, "nginx") &&
		hasContent(ctx, r, "/etc/nginx/nginx.conf", conf) &&
		ready(ctx, r, "[ -e /etc/nginx/sites-enabled/000-terangahost-default ] && [ -d /var/www/letsencrypt ]"), nil
}

func (s *StepWebServer) Execute(ctx context.Context, r domain.Runner, _ *domain.Server) error {
	if err := runAll(ctx, r, "installation de Nginx", aptInstall("nginx")); err != nil {
		return err
	}
	conf, err := templates.Static("nginx/nginx.conf")
	if err != nil {
		return err
	}
	def, err := templates.Static("nginx/default.conf")
	if err != nil {
		return err
	}
	if err := r.Upload(ctx, conf, "/etc/nginx/nginx.conf", 0o644); err != nil {
		return err
	}
	if err := r.Upload(ctx, def, "/etc/nginx/sites-available/000-terangahost-default", 0o644); err != nil {
		return err
	}
	return runAll(ctx, r, "configuration de Nginx",
		"ln -sfn /etc/nginx/sites-available/000-terangahost-default /etc/nginx/sites-enabled/000-terangahost-default",
		"rm -f /etc/nginx/sites-enabled/default",
		"install -d -m 755 /var/www /var/www/letsencrypt",
		"nginx -t",
		"systemctl enable nginx >/dev/null 2>&1 || true",
		"systemctl reload-or-restart nginx",
	)
}
