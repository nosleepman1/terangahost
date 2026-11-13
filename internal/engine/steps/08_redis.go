package steps

import (
	"context"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// StepRedis installe Redis (cache, sessions et queues), indépendamment de la base de données choisie.
type StepRedis struct{}

func (s *StepRedis) ID() string    { return "08_redis" }
func (s *StepRedis) Title() string { return "Redis (cache, sessions, queues)" }

func (s *StepRedis) PreCheck(ctx context.Context, r domain.Runner, srv *domain.Server) (bool, error) {
	if !srv.WithRedis {
		return true, nil
	}
	return packagesInstalled(ctx, r, "redis-server") && ready(ctx, r, "systemctl is-active --quiet redis-server"), nil
}

func (s *StepRedis) Execute(ctx context.Context, r domain.Runner, srv *domain.Server) error {
	if !srv.WithRedis {
		return nil
	}
	return runAll(ctx, r, "installation de Redis",
		aptInstall("redis-server"),
		"systemctl enable redis-server >/dev/null 2>&1 || true",
		"systemctl restart redis-server",
	)
}
