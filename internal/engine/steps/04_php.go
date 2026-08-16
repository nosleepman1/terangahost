package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepPHP installe le PPA de Ondřej Surý, PHP-FPM, les 14 extensions indispensables de Laravel, et optimise php.ini
type StepPHP struct{}

func (s *StepPHP) ID() string {
	return "04_php"
}

func (s *StepPHP) Title() string {
	return "Installation de PHP (PPA Ondřej Surý, PHP-FPM, 14 Extensions & OPcache/JIT)"
}

func (s *StepPHP) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	checkCmd := fmt.Sprintf("php%s -v 2>/dev/null && php%s -m | grep -q 'bcmath' && echo 'INSTALLED'", srv.PHPVersion, srv.PHPVersion)
	out, err := r.RunSilent(ctx, checkCmd)
	if err == nil && strings.Contains(out, "INSTALLED") {
		return true, nil
	}
	return false, nil
}

func (s *StepPHP) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	v := srv.PHPVersion
	if v == "" {
		v = "8.3"
	}

	// 1. Ajout du PPA Ondřej Surý
	setupPpa := []string{
		"add-apt-repository -y ppa:ondrej/php",
		"apt-get update -y",
	}
	for _, cmd := range setupPpa {
		if _, err := r.RunSilent(ctx, cmd); err != nil {
			return fmt.Errorf("erreur PPA PHP: %w", err)
		}
	}

	// 2. Les 14 extensions obligatoires pour Laravel 11/12 (Reverb, Horizon, Pulse)
	extensions := []string{
		"cli", "fpm", "common", "mysql", "pgsql", "sqlite3", "redis",
		"bcmath", "curl", "mbstring", "xml", "zip", "intl", "gd", "soap",
		"readline", "imagick", "opcache",
	}

	packages := make([]string, len(extensions))
	for i, ext := range extensions {
		packages[i] = fmt.Sprintf("php%s-%s", v, ext)
	}

	installCmd := fmt.Sprintf("apt-get install -y %s", strings.Join(packages, " "))
	if _, err := r.RunSilent(ctx, installCmd); err != nil {
		return fmt.Errorf("erreur installation paquets PHP %s: %w", v, err)
	}

	// 3. Optimisation de php.ini (CLI et FPM) pour les APIs Laravel
	phpIniOptimizations := `
upload_max_filesize = 64M
post_max_size = 64M
memory_limit = 256M
max_execution_time = 60
date.timezone = UTC
opcache.enable = 1
opcache.enable_cli = 1
opcache.memory_consumption = 128
opcache.interned_strings_buffer = 16
opcache.max_accelerated_files = 10000
opcache.validate_timestamps = 1
opcache.revalidate_freq = 0
opcache.save_comments = 1
opcache.jit = tracing
opcache.jit_buffer_size = 32M
`
	iniPath := fmt.Sprintf("/etc/php/%s/mods-available/99-terangahost.ini", v)
	if err := r.Upload(ctx, []byte(phpIniOptimizations), iniPath, 0644); err != nil {
		return fmt.Errorf("impossible de téléverser le php.ini optimisé: %w", err)
	}

	// Activer le module de configuration pour FPM et CLI
	_, _ = r.RunSilent(ctx, fmt.Sprintf("phpenmod -v %s 99-terangahost", v))
	_, _ = r.RunSilent(ctx, fmt.Sprintf("systemctl restart php%s-fpm", v))

	return nil
}
