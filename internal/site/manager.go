package site

import (
	"context"
	"fmt"
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/engine/steps"
	"github.com/nosleepman1/terangahost/internal/shell"
)

// Manager applique la configuration d'un site via un runner d'administration (root).
type Manager struct {
	R   domain.Runner
	Srv *domain.Server
}

func (m *Manager) run(ctx context.Context, what string, cmds ...string) error {
	for _, c := range cmds {
		if _, err := m.R.RunSilent(ctx, c); err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
	}
	return nil
}

// asDeployer construit une commande exécutée par le deployer depuis un dossier donné.
func asDeployer(dir, cmd string) string {
	return "cd " + shell.Quote(dir) + " && runuser -u deployer -- " + cmd
}

// EnsurePHP installe la version PHP du site si elle est absente du serveur.
func (m *Manager) EnsurePHP(ctx context.Context, version string) (installed bool, err error) {
	step := &steps.StepPHP{Version: version}
	ok, err := step.PreCheck(ctx, m.R, m.Srv)
	if err == nil && ok {
		return false, nil
	}
	return true, step.Execute(ctx, m.R, m.Srv)
}

// PrepareDirectories crée l'arborescence zero-downtime (releases/, shared/, storage partagé).
func (m *Manager) PrepareDirectories(ctx context.Context, s *domain.Site) error {
	d := shell.Quote(s.Directory)
	storage := s.Directory + "/shared/storage"
	// Chaque niveau est listé explicitement : install -d crée sinon les parents manquants en root.
	dirs := []string{
		s.Directory + "/releases",
		s.Directory + "/shared",
		storage,
		storage + "/app",
		storage + "/app/public",
		storage + "/framework",
		storage + "/framework/cache",
		storage + "/framework/cache/data",
		storage + "/framework/sessions",
		storage + "/framework/views",
		storage + "/logs",
	}
	return m.run(ctx, "création de l'arborescence",
		"install -d -m 750 -o deployer -g www-data "+d,
		"install -d -m 750 -o deployer -g www-data "+shell.Join(dirs...),
	)
}

// EnvExists indique si le fichier .env partagé existe déjà.
func (m *Manager) EnvExists(ctx context.Context, s *domain.Site) (bool, error) {
	return m.R.FileExists(ctx, s.Directory+"/shared/.env")
}

// EnsureEnv crée la base de données et le fichier .env si celui-ci n'existe pas encore.
// Un .env existant n'est jamais modifié.
func (m *Manager) EnsureEnv(ctx context.Context, s *domain.Site, createDB, ssl bool) (created bool, err error) {
	exists, err := m.EnvExists(ctx, s)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	db := DBCredentials{Connection: "sqlite"}
	if createDB && m.Srv.Database != "none" {
		db, err = m.CreateDatabase(ctx, DBName(s.Domain))
		if err != nil {
			return false, err
		}
		s.DBName = db.Name
	} else {
		sqlite := s.Directory + "/shared/database.sqlite"
		if err := m.run(ctx, "création de la base SQLite",
			"[ -f "+shell.Quote(sqlite)+" ] || install -m 640 -o deployer -g www-data /dev/null "+shell.Quote(sqlite)); err != nil {
			return false, err
		}
	}

	env, err := RenderEnv(s, m.Srv, db, ssl)
	if err != nil {
		return false, err
	}
	path := s.Directory + "/shared/.env"
	if err := m.R.Upload(ctx, env, path, 0o640); err != nil {
		return false, err
	}
	return true, m.run(ctx, "droits du .env", "chown deployer:www-data "+shell.Quote(path))
}

// CreateDatabase crée (ou réinitialise le mot de passe de) la base et l'utilisateur dédiés au site.
// Le SQL, qui contient le mot de passe, est transmis sur stdin et n'est jamais journalisé.
func (m *Manager) CreateDatabase(ctx context.Context, name string) (DBCredentials, error) {
	password := randomHex(16)
	switch m.Srv.Database {
	case "mariadb":
		sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%[1]s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n"+
			"CREATE USER IF NOT EXISTS '%[1]s'@'localhost' IDENTIFIED BY '%[2]s';\n"+
			"ALTER USER '%[1]s'@'localhost' IDENTIFIED BY '%[2]s';\n"+
			"GRANT ALL PRIVILEGES ON `%[1]s`.* TO '%[1]s'@'localhost';\n"+
			"FLUSH PRIVILEGES;\n", name, password)
		if _, err := m.R.RunWithInput(ctx, "mysql --protocol=socket -uroot", []byte(sql)); err != nil {
			return DBCredentials{}, fmt.Errorf("création de la base MariaDB: %w", err)
		}
		return DBCredentials{Connection: "mysql", Port: 3306, Name: name, User: name, Password: password}, nil
	case "postgres":
		sql := fmt.Sprintf("DO $$BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%[1]s') THEN CREATE ROLE \"%[1]s\" LOGIN; END IF; END$$;\n"+
			"ALTER ROLE \"%[1]s\" WITH LOGIN PASSWORD '%[2]s';\n"+
			"SELECT 'CREATE DATABASE \"%[1]s\" OWNER \"%[1]s\"' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '%[1]s')\\gexec\n",
			name, password)
		if _, err := m.R.RunWithInput(ctx, "cd /tmp && runuser -u postgres -- psql -v ON_ERROR_STOP=1 -q", []byte(sql)); err != nil {
			return DBCredentials{}, fmt.Errorf("création de la base PostgreSQL: %w", err)
		}
		return DBCredentials{Connection: "pgsql", Port: 5432, Name: name, User: name, Password: password}, nil
	}
	return DBCredentials{}, fmt.Errorf("aucune base de données sur ce serveur")
}

// ConfigureFPM installe le pool PHP-FPM dédié au site (socket Unix et limites propres).
func (m *Manager) ConfigureFPM(ctx context.Context, s *domain.Site, maxChildren int) error {
	pool, err := RenderFPMPool(NewConfigData(s, false, maxChildren))
	if err != nil {
		return err
	}
	v := s.PHPVersion
	path := fpmPool(v, s.ID)
	if err := m.backup(ctx, path); err != nil {
		return err
	}
	if err := m.R.Upload(ctx, pool, path, 0o644); err != nil {
		return err
	}
	if _, err := m.R.RunSilent(ctx, fmt.Sprintf("php-fpm%s -t", v)); err != nil {
		_ = m.restore(ctx, path)
		return fmt.Errorf("pool PHP-FPM invalide, modification annulée: %w", err)
	}
	return m.run(ctx, "activation du pool PHP-FPM",
		"rm -f "+shell.Quote(path+".bak"),
		// Le pool "www" par défaut est inutile et consomme de la mémoire.
		fmt.Sprintf("[ ! -f /etc/php/%[1]s/fpm/pool.d/www.conf ] || mv -f /etc/php/%[1]s/fpm/pool.d/www.conf /etc/php/%[1]s/fpm/pool.d/www.conf.disabled", v),
		// Si le site change de version PHP, son ancien pool est retiré.
		fmt.Sprintf(`for f in /etc/php/*/fpm/pool.d/%[1]s.conf; do [ -e "$f" ] && [ "$f" != %[2]s ] || continue; rm -f "$f"; ov=$(echo "$f" | cut -d/ -f4); systemctl reload "php$ov-fpm" || true; done`,
			s.ID, shell.Quote(path)),
		fmt.Sprintf("systemctl reload-or-restart php%s-fpm", v),
	)
}

// CertificateExists indique si un certificat Let's Encrypt existe pour le domaine.
func (m *Manager) CertificateExists(ctx context.Context, d string) bool {
	ok, err := m.R.FileExists(ctx, certPath(d))
	return err == nil && ok
}

// ConfigureNginx installe le vhost du site (HTTPS si un certificat existe) et recharge Nginx.
// En cas d'erreur de syntaxe, la configuration précédente est restaurée.
func (m *Manager) ConfigureNginx(ctx context.Context, s *domain.Site, maxChildren int) (ssl bool, err error) {
	ssl = m.CertificateExists(ctx, s.Domain)
	conf, err := RenderNginx(NewConfigData(s, ssl, maxChildren))
	if err != nil {
		return false, err
	}
	avail, enabled := nginxAvailable(s.Domain), nginxEnabled(s.Domain)
	if err := m.backup(ctx, avail); err != nil {
		return false, err
	}
	if err := m.R.Upload(ctx, conf, avail, 0o644); err != nil {
		return false, err
	}
	if _, err := m.R.RunSilent(ctx, "ln -sfn "+shell.Quote(avail)+" "+shell.Quote(enabled)+" && nginx -t"); err != nil {
		if restored := m.restore(ctx, avail); !restored {
			_, _ = m.R.RunSilent(ctx, "rm -f "+shell.Quote(enabled))
		}
		return false, fmt.Errorf("configuration Nginx invalide, modification annulée: %w", err)
	}
	return ssl, m.run(ctx, "rechargement de Nginx", "rm -f "+shell.Quote(avail+".bak"), "systemctl reload nginx")
}

// IssueCertificate obtient un certificat Let's Encrypt (validation webroot, sans modifier Nginx).
func (m *Manager) IssueCertificate(ctx context.Context, s *domain.Site, email string) error {
	args := []string{"certbot", "certonly", "--webroot", "-w", "/var/www/letsencrypt",
		"--non-interactive", "--agree-tos", "--keep-until-expiring", "--cert-name", s.Domain}
	if email != "" {
		args = append(args, "--email", email)
	} else {
		args = append(args, "--register-unsafely-without-email")
	}
	for _, d := range append([]string{s.Domain}, s.Aliases...) {
		args = append(args, "-d", d)
	}
	if _, err := m.R.RunSilent(ctx, shell.Join(args...)); err != nil {
		return fmt.Errorf("Certbot: %w", err)
	}
	return nil
}

// SwitchEnvToHTTPS met à jour APP_URL, SESSION_SECURE_COOKIE et REVERB_* dans le .env après l'activation du SSL.
func (m *Manager) SwitchEnvToHTTPS(ctx context.Context, s *domain.Site) error {
	env := shell.Quote(s.Directory + "/shared/.env")
	return m.run(ctx, "mise à jour du .env",
		"[ ! -f "+env+" ] || sed -i -e 's#^APP_URL=http://#APP_URL=https://#' -e 's#^SESSION_SECURE_COOKIE=false#SESSION_SECURE_COOKIE=true#' "+
			"-e 's#^REVERB_PORT=80$#REVERB_PORT=443#' -e 's#^REVERB_SCHEME=http$#REVERB_SCHEME=https#' "+env)
}

// RefreshApp recharge la configuration de la release active (après une modification du .env).
func (m *Manager) RefreshApp(ctx context.Context, s *domain.Site) error {
	current := s.Directory + "/current"
	php := "php" + s.PHPVersion
	return m.run(ctx, "rechargement de l'application",
		"[ ! -f "+shell.Quote(current+"/artisan")+" ] || { "+asDeployer(current, php+" artisan config:cache")+" && "+
			asDeployer(current, php+" artisan queue:restart")+"; }",
		fmt.Sprintf("systemctl reload php%s-fpm", s.PHPVersion),
	)
}

// ConfigureWorkers installe ou retire les programmes Supervisor (queues et Reverb).
func (m *Manager) ConfigureWorkers(ctx context.Context, s *domain.Site) error {
	d := NewConfigData(s, false, 0)
	if s.QueueWorkers > 0 {
		conf, err := RenderWorker(d)
		if err != nil {
			return err
		}
		if err := m.R.Upload(ctx, conf, supervisorWorker(s.ID), 0o644); err != nil {
			return err
		}
	} else if err := m.run(ctx, "suppression des workers", "rm -f "+supervisorWorker(s.ID)); err != nil {
		return err
	}
	if s.WithReverb {
		conf, err := RenderReverb(d)
		if err != nil {
			return err
		}
		if err := m.R.Upload(ctx, conf, supervisorReverb(s.ID), 0o644); err != nil {
			return err
		}
	} else if err := m.run(ctx, "suppression de Reverb", "rm -f "+supervisorReverb(s.ID)); err != nil {
		return err
	}
	return m.run(ctx, "mise à jour de Supervisor", "supervisorctl reread >/dev/null && supervisorctl update")
}

// ConfigureScheduler installe ou retire la tâche cron du scheduler Laravel.
func (m *Manager) ConfigureScheduler(ctx context.Context, s *domain.Site) error {
	if !s.Scheduler {
		return m.run(ctx, "suppression du scheduler", "rm -f "+cronFile(s.ID))
	}
	conf, err := RenderScheduler(NewConfigData(s, false, 0))
	if err != nil {
		return err
	}
	return m.R.Upload(ctx, conf, cronFile(s.ID), 0o644)
}

// DeleteOptions contrôle ce qui est supprimé avec un site.
type DeleteOptions struct {
	PurgeFiles   bool // supprime /var/www/<domaine> et le certificat
	DropDatabase bool // supprime la base et l'utilisateur
}

// Delete retire la configuration du site du serveur.
func (m *Manager) Delete(ctx context.Context, s *domain.Site, opts DeleteOptions) error {
	if err := m.run(ctx, "suppression du vhost Nginx",
		"rm -f "+shell.Join(nginxEnabled(s.Domain), nginxAvailable(s.Domain), nginxAvailable(s.Domain)+".bak"),
		"nginx -t && systemctl reload nginx",
		"rm -f "+shell.Join(supervisorWorker(s.ID), supervisorReverb(s.ID), cronFile(s.ID)),
		"supervisorctl reread >/dev/null && supervisorctl update",
		// Retire le pool ; si plus aucun pool n'existe, réactive "www" pour que PHP-FPM puisse démarrer.
		fmt.Sprintf(`for f in /etc/php/*/fpm/pool.d/%s.conf; do [ -e "$f" ] || continue; rm -f "$f"; d=$(dirname "$f"); `+
			`ls "$d"/*.conf >/dev/null 2>&1 || { [ ! -f "$d/www.conf.disabled" ] || mv -f "$d/www.conf.disabled" "$d/www.conf"; }; `+
			`systemctl reload "php$(echo "$f" | cut -d/ -f4)-fpm" || true; done`, s.ID),
	); err != nil {
		return err
	}
	if opts.DropDatabase && s.DBName != "" {
		switch m.Srv.Database {
		case "mariadb":
			sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%[1]s`;\nDROP USER IF EXISTS '%[1]s'@'localhost';\n", s.DBName)
			if _, err := m.R.RunWithInput(ctx, "mysql --protocol=socket -uroot", []byte(sql)); err != nil {
				return fmt.Errorf("suppression de la base: %w", err)
			}
		case "postgres":
			sql := fmt.Sprintf("DROP DATABASE IF EXISTS \"%[1]s\";\nDROP ROLE IF EXISTS \"%[1]s\";\n", s.DBName)
			if _, err := m.R.RunWithInput(ctx, "cd /tmp && runuser -u postgres -- psql -v ON_ERROR_STOP=1 -q", []byte(sql)); err != nil {
				return fmt.Errorf("suppression de la base: %w", err)
			}
		}
	}
	if opts.PurgeFiles {
		if !strings.HasPrefix(s.Directory, WebRoot+"/") || strings.Contains(s.Directory, "..") {
			return fmt.Errorf("dossier %q hors de %s : suppression refusée", s.Directory, WebRoot)
		}
		return m.run(ctx, "suppression des fichiers",
			"rm -rf -- "+shell.Quote(s.Directory),
			"certbot delete --non-interactive --cert-name "+shell.Quote(s.Domain)+" >/dev/null 2>&1 || true",
		)
	}
	return nil
}

func (m *Manager) backup(ctx context.Context, path string) error {
	p := shell.Quote(path)
	return m.run(ctx, "sauvegarde de "+path, "rm -f "+shell.Quote(path+".bak")+"; [ ! -f "+p+" ] || cp -a "+p+" "+shell.Quote(path+".bak"))
}

// restore remet la sauvegarde en place ; retourne false s'il n'y avait pas de version précédente.
func (m *Manager) restore(ctx context.Context, path string) bool {
	out, err := m.R.RunSilent(ctx, fmt.Sprintf("if [ -f %[1]s ]; then mv -f %[1]s %[2]s && echo RESTORED; else rm -f %[2]s; fi",
		shell.Quote(path+".bak"), shell.Quote(path)))
	return err == nil && strings.Contains(out, "RESTORED")
}
