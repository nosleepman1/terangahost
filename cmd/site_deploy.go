package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/deploy"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var sd struct {
	domain, server, repo, branch string
	noMigrate                    bool
	keep                         int
	timeout                      time.Duration
}

var siteDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Déploie la dernière version d'une application Laravel sans interruption de service",
	Long: `Clone le dépôt dans une nouvelle release, installe les dépendances Composer, met en cache la
configuration, exécute les migrations puis bascule atomiquement le lien 'current'.
Si une étape échoue avant la bascule, la release est supprimée et la production reste inchangée.
Le dépôt et la branche sont mémorisés : les déploiements suivants ne nécessitent que --domain.`,
	Example: `  terangahost site deploy --domain=api.exemple.com --repo=git@github.com:org/api.git --branch=main
  terangahost site deploy --domain=api.exemple.com`,
	RunE: func(*cobra.Command, []string) error { return runSiteDeploy() },
}

func init() {
	f := siteDeployCmd.Flags()
	f.StringVar(&sd.domain, "domain", "", "Domaine du site")
	f.StringVar(&sd.server, "server", "", "Nom du serveur (vérification facultative)")
	f.StringVar(&sd.repo, "repo", "", "URL du dépôt Git (HTTPS ou SSH) ; mémorisée pour les prochains déploiements")
	f.StringVar(&sd.branch, "branch", "", "Branche ou tag à déployer (défaut : branche mémorisée, sinon main)")
	f.BoolVar(&sd.noMigrate, "no-migrate", false, "N'exécute pas les migrations")
	f.IntVar(&sd.keep, "keep", 5, "Nombre de releases conservées pour le retour arrière")
	f.DurationVar(&sd.timeout, "timeout", 30*time.Minute, "Durée maximale du déploiement")
	_ = siteDeployCmd.MarkFlagRequired("domain")
	siteCmd.AddCommand(siteDeployCmd)
}

func runSiteDeploy() error {
	domainName, err := validate.Domain(sd.domain)
	if err != nil {
		return err
	}
	if err := validate.Range("--keep", sd.keep, 1, 50); err != nil {
		return err
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	ctx, cancel := commandContext(sd.timeout)
	defer cancel()
	s, srv, err := siteWithServer(ctx, store, domainName, sd.server)
	if err != nil {
		return err
	}

	repo, branch := firstNonEmpty(sd.repo, s.Repository), firstNonEmpty(sd.branch, s.Branch, "main")
	if repo == "" {
		return fmt.Errorf("aucun dépôt mémorisé pour %s : précisez --repo", domainName)
	}
	if err := validate.Repository(repo); err != nil {
		return err
	}
	if err := validate.Branch(branch); err != nil {
		return err
	}

	log := openLog("deploy_" + s.ID)
	defer log.Close()
	r, err := connectDeployer(srv, log)
	if err != nil {
		return err
	}
	defer r.Close()

	ui.Info("Déploiement de %s %s sur %s", ui.Cyan(domainName), ui.Gray("("+repo+"@"+branch+")"), srv.Name)
	start := time.Now()
	var sp *ui.Spinner
	var current string
	res, err := deploy.Run(ctx, r, s, deploy.Options{Repository: repo, Branch: branch, Migrate: !sd.noMigrate, KeepReleases: sd.keep}, deploy.Events{
		OnStep: func(title string) {
			if sp != nil {
				sp.Succeed(current)
			}
			current = title
			sp = ui.StartSpinner(title)
		},
		OnWarn: func(msg string) {
			if sp != nil {
				sp.Update(current + ui.Yellow(" ! "+msg))
			}
		},
		OnOutput: func(line string) {
			if verbose {
				fmt.Println(ui.Gray("    " + line))
			}
		},
	})
	if err != nil {
		if sp != nil {
			sp.Fail(current)
		}
		if res != nil && len(res.Output) > 0 && !verbose {
			ui.Info("\n%s", ui.Gray(strings.Join(lastN(res.Output, 25), "\n")))
		}
		if res != nil && res.Activated {
			ui.Warn("la nouvelle release %s est active mais une étape finale a échoué", res.Release)
		} else {
			ui.Warn("déploiement annulé : la version en production est inchangée")
		}
		if res != nil && gitAccessDenied(res.Output) {
			ui.Info("\nAccès au dépôt refusé. Pour un dépôt privé, ajoutez la deploy key du serveur :")
			ui.Hint("terangahost server deploy-key --name=" + srv.Name)
		}
		if log != nil {
			ui.Info("Journal détaillé : %s", log.Path)
		}
		return fmt.Errorf("échec du déploiement de %s: %w", domainName, err)
	}
	if sp != nil {
		sp.Succeed(current)
	}

	s.Repository, s.Branch, s.CurrentRelease, s.LastCommit, s.LastDeployAt = repo, branch, res.Release, res.Commit, time.Now()
	if err := store.SaveSite(ctx, s); err != nil {
		return err
	}

	scheme := "http"
	if s.HasSSL {
		scheme = "https"
	}
	fmt.Println()
	ui.Rule()
	ui.Success("Déploiement terminé en %s", time.Since(start).Round(time.Second))
	ui.KV("URL", scheme+"://"+domainName)
	ui.KV("Release", res.Release)
	ui.KV("Commit", res.Commit)
	for _, w := range res.Warnings {
		ui.Warn("%s", w)
	}
	ui.Rule()
	ui.Info("Retour arrière si nécessaire : terangahost site rollback --domain=%s", domainName)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func lastN(lines []string, n int) []string {
	if len(lines) > n {
		return lines[len(lines)-n:]
	}
	return lines
}

func gitAccessDenied(output []string) bool {
	joined := strings.ToLower(strings.Join(output, "\n"))
	for _, s := range []string{"permission denied (publickey)", "could not read from remote repository", "repository not found", "authentication failed", "could not read username"} {
		if strings.Contains(joined, s) {
			return true
		}
	}
	return false
}
