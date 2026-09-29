package steps

import (
	"context"
	"fmt"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/templates"
)

const sudoersPath = "/etc/sudoers.d/terangahost-deployer"

// StepSudoers installe la règle sudo minimale du deployer : uniquement le rechargement de PHP-FPM.
// Elle remplace la règle des versions précédentes (service/supervisorctl/certbot avec arguments
// libres) qui permettait une élévation de privilèges vers root.
type StepSudoers struct{}

func (s *StepSudoers) ID() string { return "03_sudoers" }
func (s *StepSudoers) Title() string {
	return "Privilèges sudo minimaux du deployer (moindre privilège)"
}

func (s *StepSudoers) PreCheck(ctx context.Context, r domain.Runner, _ *domain.Server) (bool, error) {
	want, err := templates.Static("sudoers/deployer")
	if err != nil {
		return false, err
	}
	return hasContent(ctx, r, sudoersPath, want), nil
}

func (s *StepSudoers) Execute(ctx context.Context, r domain.Runner, _ *domain.Server) error {
	content, err := templates.Static("sudoers/deployer")
	if err != nil {
		return err
	}
	// Les fichiers contenant un point sont ignorés par sudo : le brouillon est donc inactif.
	tmp := sudoersPath + ".new"
	if err := r.Upload(ctx, content, tmp, 0o440); err != nil {
		return err
	}
	if _, err := r.RunSilent(ctx, "visudo -cf "+tmp); err != nil {
		_, _ = r.RunSilent(ctx, "rm -f "+tmp)
		return fmt.Errorf("règle sudoers invalide: %w", err)
	}
	return runAll(ctx, r, "installation de la règle sudoers", "mv -f "+tmp+" "+sudoersPath)
}
