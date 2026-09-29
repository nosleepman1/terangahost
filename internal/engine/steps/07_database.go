package steps

import (
	"context"
	"fmt"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// StepDatabase installe MariaDB ou PostgreSQL avec un réglage mémoire adapté à la RAM réelle.
type StepDatabase struct{}

func (s *StepDatabase) ID() string { return "07_database" }

func (s *StepDatabase) Title() string {
	return "Base de données (réglage mémoire selon la RAM réelle)"
}

const (
	mariadbTunePath = "/etc/mysql/mariadb.conf.d/99-terangahost.cnf"
	pgTunedMarker   = "/etc/terangahost/postgres-tuned"
)

func (s *StepDatabase) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	switch srv.Database {
	case "mariadb":
		return packagesInstalled(ctx, r, "mariadb-server") && ready(ctx, r, "[ -f "+mariadbTunePath+" ]"), nil
	case "postgres":
		return packagesInstalled(ctx, r, "postgresql") && ready(ctx, r, "[ -f "+pgTunedMarker+" ]"), nil
	default:
		return true, nil
	}
}

func (s *StepDatabase) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	switch srv.Database {
	case "mariadb":
		return s.mariadb(ctx, r, srv)
	case "postgres":
		return s.postgres(ctx, r, srv)
	}
	return nil
}

func (s *StepDatabase) mariadb(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	if err := runAll(ctx, r, "installation de MariaDB", aptInstall("mariadb-server", "mariadb-client")); err != nil {
		return err
	}
	tune := fmt.Sprintf(`# Managed by TerangaHost
[mysqld]
bind-address = 127.0.0.1
innodb_buffer_pool_size = %dM
innodb_flush_log_at_trx_commit = 2
max_connections = 150
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
`, srv.Hardware.TunedMySQLBufferPoolMB())
	if err := r.Upload(ctx, []byte(tune), mariadbTunePath, 0o644); err != nil {
		return err
	}
	if err := runAll(ctx, r, "démarrage de MariaDB",
		"systemctl enable mariadb >/dev/null 2>&1 || true",
		"systemctl restart mariadb",
	); err != nil {
		return err
	}
	// Équivalent de mysql_secure_installation : root reste authentifié par socket Unix.
	secure := "DELETE FROM mysql.global_priv WHERE User='';\nDROP DATABASE IF EXISTS test;\nDELETE FROM mysql.db WHERE Db='test' OR Db='test\\\\_%';\nFLUSH PRIVILEGES;\n"
	if _, err := r.RunWithInput(ctx, "mysql --protocol=socket -uroot", []byte(secure)); err != nil {
		return fmt.Errorf("sécurisation de MariaDB: %w", err)
	}
	return nil
}

func (s *StepDatabase) postgres(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	if err := runAll(ctx, r, "installation de PostgreSQL",
		aptInstall("postgresql", "postgresql-contrib"),
		"systemctl enable --now postgresql",
	); err != nil {
		return err
	}
	ram := srv.Hardware.TotalRAMMB
	if ram <= 0 {
		ram = 1024
	}
	tune := fmt.Sprintf("ALTER SYSTEM SET shared_buffers = '%dMB';\nALTER SYSTEM SET effective_cache_size = '%dMB';\n", ram/4, ram/2)
	if _, err := r.RunWithInput(ctx, "cd /tmp && runuser -u postgres -- psql -v ON_ERROR_STOP=1 -q", []byte(tune)); err != nil {
		return fmt.Errorf("réglage de PostgreSQL: %w", err)
	}
	return runAll(ctx, r, "redémarrage de PostgreSQL",
		"systemctl restart postgresql",
		"install -d -m 755 /etc/terangahost && touch "+pgTunedMarker,
	)
}
