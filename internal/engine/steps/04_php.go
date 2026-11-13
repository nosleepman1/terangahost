package steps

import (
	"context"
	"fmt"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/templates"
)

// PHPExtensions liste les extensions installées pour Laravel 10/11/12 (Horizon, Reverb, Pulse...).
var PHPExtensions = []string{
	"cli", "fpm", "common", "mysql", "pgsql", "sqlite3", "redis",
	"bcmath", "curl", "mbstring", "xml", "zip", "intl", "gd", "soap",
	"readline", "imagick", "opcache",
}

// PHPPackages retourne les paquets APT d'une version PHP.
func PHPPackages(version string) []string {
	pkgs := make([]string, len(PHPExtensions))
	for i, ext := range PHPExtensions {
		pkgs[i] = fmt.Sprintf("php%s-%s", version, ext)
	}
	return pkgs
}

// StepPHP installe PHP-FPM depuis le PPA ondrej/php avec les extensions Laravel et un php.ini optimisé.
type StepPHP struct {
	Version string
}

func (s *StepPHP) ID() string { return "04_php_" + s.Version }

func (s *StepPHP) Title() string {
	return fmt.Sprintf("PHP %s-FPM, %d extensions Laravel, OPcache et JIT", s.Version, len(PHPExtensions))
}

func (s *StepPHP) iniPath() string {
	return fmt.Sprintf("/etc/php/%s/mods-available/terangahost.ini", s.Version)
}

func (s *StepPHP) PreCheck(ctx context.Context, r domain.Runner, _ *domain.Server) (bool, error) {
	ini, err := templates.Static("php/laravel.ini")
	if err != nil {
		return false, err
	}
	return packagesInstalled(ctx, r, PHPPackages(s.Version)...) && hasContent(ctx, r, s.iniPath(), ini), nil
}

func (s *StepPHP) Execute(ctx context.Context, r domain.Runner, _ *domain.Server) error {
	v := s.Version
	if err := runAll(ctx, r, "dépôt PHP ondrej/php",
		"grep -rqs 'ondrej/php' /etc/apt/sources.list.d/ || add-apt-repository -y ppa:ondrej/php",
		aptUpdate(),
	); err != nil {
		return err
	}
	if err := runAll(ctx, r, "installation de PHP "+v, aptInstall(PHPPackages(v)...)); err != nil {
		return err
	}

	ini, err := templates.Static("php/laravel.ini")
	if err != nil {
		return err
	}
	if err := r.Upload(ctx, ini, s.iniPath(), 0o644); err != nil {
		return err
	}
	return runAll(ctx, r, "configuration de PHP "+v,
		// Nettoyage du module des versions précédentes de TerangaHost.
		fmt.Sprintf("phpdismod -v %[1]s 99-terangahost >/dev/null 2>&1 || true; rm -f /etc/php/%[1]s/mods-available/99-terangahost.ini", v),
		fmt.Sprintf("phpenmod -v %s terangahost", v),
		fmt.Sprintf("systemctl enable php%s-fpm >/dev/null 2>&1 || true", v),
		fmt.Sprintf("php-fpm%[1]s -t >/dev/null 2>&1 && systemctl restart php%[1]s-fpm", v),
	)
}
