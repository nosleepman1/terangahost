package cmd

import (
	"fmt"
	"time"

	"github.com/nosleepman1/terangahost/internal/deploy"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var sr struct {
	domain, release string
	list            bool
}

var siteRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Revient instantanément à la release précédente (ou à --release)",
	Long: `Bascule le lien 'current' vers une release antérieure déjà installée, sans rien reconstruire.
Attention : les migrations de base de données ne sont pas annulées.`,
	RunE: func(*cobra.Command, []string) error {
		domainName, err := validate.Domain(sr.domain)
		if err != nil {
			return err
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		ctx, cancel := commandContext(5 * time.Minute)
		defer cancel()
		s, srv, err := siteWithServer(ctx, store, domainName, "")
		if err != nil {
			return err
		}
		r, err := connectDeployer(srv, nil)
		if err != nil {
			return err
		}
		defer r.Close()

		releases, current, err := deploy.Releases(ctx, r, s)
		if err != nil {
			return err
		}
		if sr.list {
			if len(releases) == 0 {
				ui.Info("Aucune release déployée.")
			}
			for _, rel := range releases {
				marker := "  "
				if rel == current {
					marker = ui.Green("→ ")
				}
				t, _ := time.Parse("20060102150405", rel)
				ui.Info("%s%s  %s", marker, rel, ui.Gray(t.Local().Format("02/01/2006 15:04:05")))
			}
			return nil
		}

		target := sr.release
		if target == "" {
			if target, err = deploy.PreviousRelease(releases, current); err != nil {
				return err
			}
		}
		if target == current {
			return fmt.Errorf("la release %s est déjà active", target)
		}
		if err := task(fmt.Sprintf("Activation de la release %s", target), func() error {
			return deploy.Activate(ctx, r, s, target)
		}); err != nil {
			return err
		}
		s.CurrentRelease = target
		if err := store.SaveSite(ctx, s); err != nil {
			return err
		}
		ui.Success("%s tourne maintenant sur la release %s (précédente : %s)", domainName, target, current)
		ui.Warn("les migrations de base de données n'ont pas été annulées")
		return nil
	},
}

func init() {
	siteRollbackCmd.Flags().StringVar(&sr.domain, "domain", "", "Domaine du site")
	siteRollbackCmd.Flags().StringVar(&sr.release, "release", "", "Release à activer (défaut : la précédente)")
	siteRollbackCmd.Flags().BoolVar(&sr.list, "list", false, "Liste les releases disponibles")
	_ = siteRollbackCmd.MarkFlagRequired("domain")
	siteCmd.AddCommand(siteRollbackCmd)
}
