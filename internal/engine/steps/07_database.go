package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// StepDatabase configure MariaDB ou PostgreSQL et Redis avec tuning automatique de la mémoire selon la RAM réelle
type StepDatabase struct{}

func (s *StepDatabase) ID() string {
	return "07_database"
}

func (s *StepDatabase) Title() string {
	return "Installation et optimisation de la Base de Données (Hardware-Aware Memory Tuning) & Redis"
}

func (s *StepDatabase) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	if srv.Database == "none" || srv.Database == "" {
		return true, nil
	}

	if srv.Database == "mariadb" || srv.Database == "mysql" {
		out, err := r.RunSilent(ctx, "mysql --version 2>/dev/null && echo 'INSTALLED'")
		if err == nil && strings.Contains(out, "INSTALLED") {
			return true, nil
		}
	} else if srv.Database == "postgres" || srv.Database == "postgresql" {
		out, err := r.RunSilent(ctx, "psql --version 2>/dev/null && echo 'INSTALLED'")
		if err == nil && strings.Contains(out, "INSTALLED") {
			return true, nil
		}
	}

	return false, nil
}

func (s *StepDatabase) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	// 1. Installation de Redis si demandé
	if srv.WithRedis {
		redisCommands := []string{
			"apt-get install -y redis-server",
			"systemctl enable redis-server",
			"systemctl start redis-server",
		}
		for _, cmd := range redisCommands {
			if _, err := r.RunSilent(ctx, cmd); err != nil {
				return fmt.Errorf("erreur installation Redis: %w", err)
			}
		}
	}

	// 2. Base de données
	if srv.Database == "mariadb" || srv.Database == "mysql" {
		if _, err := r.RunSilent(ctx, "apt-get install -y mariadb-server mariadb-client"); err != nil {
			return fmt.Errorf("erreur installation MariaDB: %w", err)
		}

		// Tuning mémoire dynamique : calcul du pool buffer idéal
		bufferPoolMB := srv.Hardware.TunedMySQLBufferPoolMB()
		if bufferPoolMB == 0 {
			bufferPoolMB = 128
		}

		mariadbTune := fmt.Sprintf(`[mysqld]
innodb_buffer_pool_size = %dM
innodb_log_file_size = 32M
innodb_flush_log_at_trx_commit = 2
max_connections = 100
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
`, bufferPoolMB)

		_ = r.Upload(ctx, []byte(mariadbTune), "/etc/mysql/mariadb.conf.d/99-terangahost.cnf", 0644)
		_, _ = r.RunSilent(ctx, "systemctl restart mariadb")

	} else if srv.Database == "postgres" || srv.Database == "postgresql" {
		if _, err := r.RunSilent(ctx, "apt-get install -y postgresql postgresql-contrib"); err != nil {
			return fmt.Errorf("erreur installation PostgreSQL: %w", err)
		}
		_, _ = r.RunSilent(ctx, "systemctl enable postgresql && systemctl start postgresql")
	}

	return nil
}
