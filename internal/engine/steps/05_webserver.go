package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepWebServer installe et optimise Nginx pour les API Laravel (buffers rapides, WebSockets, sécurité)
type StepWebServer struct{}

func (s *StepWebServer) ID() string {
	return "05_webserver"
}

func (s *StepWebServer) Title() string {
	return "Installation et optimisation de Nginx (FastCGI Buffers & WebSockets)"
}

func (s *StepWebServer) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	out, err := r.RunSilent(ctx, "nginx -v 2>&1")
	if err == nil && strings.Contains(out, "nginx version") {
		return true, nil
	}
	return false, nil
}

func (s *StepWebServer) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	commands := []string{
		"apt-get install -y nginx",
		"systemctl enable nginx",
		"rm -f /etc/nginx/sites-enabled/default",
	}

	for _, cmd := range commands {
		if _, err := r.RunSilent(ctx, cmd); err != nil {
			return fmt.Errorf("erreur installation Nginx: %w", err)
		}
	}

	// Configuration globale Nginx optimisée pour API Laravel
	nginxConf := `user www-data;
worker_processes auto;
pid /run/nginx.pid;
include /etc/nginx/modules-enabled/*.conf;

events {
    worker_connections 1024;
    multi_accept on;
}

http {
    sendfile on;
    tcp_nopush on;
    tcp_nodelay on;
    keepalive_timeout 65;
    types_hash_max_size 2048;
    server_tokens off;

    client_max_body_size 64M;
    client_body_buffer_size 128k;

    # FastCGI Buffers pour grosses réponses JSON d'API
    fastcgi_buffers 16 16k;
    fastcgi_buffer_size 32k;

    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    # Logs
    access_log /var/log/nginx/access.log;
    error_log /var/log/nginx/error.log;

    # Gzip
    gzip on;
    gzip_vary on;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

    include /etc/nginx/conf.d/*.conf;
    include /etc/nginx/sites-enabled/*;
}
`
	if err := r.Upload(ctx, []byte(nginxConf), "/etc/nginx/nginx.conf", 0644); err != nil {
		return fmt.Errorf("impossible de téléverser /etc/nginx/nginx.conf: %w", err)
	}

	// Test de la syntaxe Nginx et rechargement
	if _, err := r.RunSilent(ctx, "nginx -t && systemctl restart nginx"); err != nil {
		return fmt.Errorf("syntaxe Nginx invalide ou échec du démarrage: %w", err)
	}

	return nil
}
