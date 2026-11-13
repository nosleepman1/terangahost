package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/shell"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var se struct {
	domain, file string
}

var siteEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Lit ou modifie le fichier .env de production d'un site",
}

var siteEnvPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Affiche le .env de production (ou l'écrit dans --file)",
	RunE: func(*cobra.Command, []string) error {
		return withSiteEnv(func(ctx context.Context, r domain.Runner, s *domain.Site) error {
			var buf bytes.Buffer
			if err := r.Execute(ctx, "cat "+shell.Quote(s.Directory+"/shared/.env"), &buf, nil); err != nil {
				return err
			}
			if se.file == "" {
				_, err := os.Stdout.Write(buf.Bytes())
				return err
			}
			if err := os.WriteFile(se.file, buf.Bytes(), 0o600); err != nil {
				return err
			}
			ui.Success(".env écrit dans %s", se.file)
			return nil
		})
	},
}

var siteEnvPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Remplace le .env de production par --file puis recharge l'application",
	RunE: func(*cobra.Command, []string) error {
		content, err := os.ReadFile(se.file)
		if err != nil {
			return err
		}
		if err := checkEnv(content); err != nil {
			return err
		}
		return withSiteEnv(func(ctx context.Context, r domain.Runner, s *domain.Site) error {
			return pushEnv(ctx, r, s, content)
		})
	},
}

var siteEnvEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Ouvre le .env de production dans $EDITOR puis l'applique",
	RunE: func(*cobra.Command, []string) error {
		return withSiteEnv(func(ctx context.Context, r domain.Runner, s *domain.Site) error {
			var buf bytes.Buffer
			if err := r.Execute(ctx, "cat "+shell.Quote(s.Directory+"/shared/.env"), &buf, nil); err != nil {
				return err
			}
			tmp, err := os.CreateTemp("", "terangahost-*.env")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			if _, err := tmp.Write(buf.Bytes()); err != nil {
				tmp.Close()
				return err
			}
			tmp.Close()

			editor := os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = "vi"
				if runtime.GOOS == "windows" {
					editor = "notepad"
				}
			}
			parts := strings.Fields(editor)
			c := exec.Command(parts[0], append(parts[1:], tmp.Name())...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("éditeur %s: %w", editor, err)
			}
			content, err := os.ReadFile(tmp.Name())
			if err != nil {
				return err
			}
			if bytes.Equal(content, buf.Bytes()) {
				ui.Info("Aucune modification.")
				return nil
			}
			if err := checkEnv(content); err != nil {
				return err
			}
			return pushEnv(ctx, r, s, content)
		})
	},
}

func init() {
	for _, c := range []*cobra.Command{siteEnvPullCmd, siteEnvPushCmd, siteEnvEditCmd} {
		c.Flags().StringVar(&se.domain, "domain", "", "Domaine du site")
		_ = c.MarkFlagRequired("domain")
	}
	siteEnvPullCmd.Flags().StringVar(&se.file, "file", "", "Fichier local de destination (défaut : sortie standard)")
	siteEnvPushCmd.Flags().StringVar(&se.file, "file", "", "Fichier .env local à envoyer")
	_ = siteEnvPushCmd.MarkFlagRequired("file")
	siteEnvCmd.AddCommand(siteEnvPullCmd, siteEnvPushCmd, siteEnvEditCmd)
	siteCmd.AddCommand(siteEnvCmd)
}

func withSiteEnv(fn func(ctx context.Context, r domain.Runner, s *domain.Site) error) error {
	domainName, err := validate.Domain(se.domain)
	if err != nil {
		return err
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	ctx, cancel := commandContext(30 * time.Minute)
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
	return fn(ctx, r, s)
}

// checkEnv refuse un fichier vide ou sans APP_KEY, qui casserait l'application en production.
func checkEnv(content []byte) error {
	text := string(content)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: le fichier .env est vide", domain.ErrInvalidInput)
	}
	for _, l := range strings.Split(text, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "APP_KEY="); ok && strings.TrimSpace(v) != "" {
			return nil
		}
	}
	return fmt.Errorf("%w: APP_KEY absente du .env", domain.ErrInvalidInput)
}

func pushEnv(ctx context.Context, r domain.Runner, s *domain.Site, content []byte) error {
	path := s.Directory + "/shared/.env"
	if err := task("Envoi du .env", func() error { return r.Upload(ctx, content, path, 0o640) }); err != nil {
		return err
	}
	current := shell.Quote(s.Directory + "/current")
	php := "php" + s.PHPVersion
	return task("Rechargement de la configuration", func() error {
		_, err := r.RunSilent(ctx, fmt.Sprintf("[ ! -f %[1]s/artisan ] || { cd %[1]s && %[2]s artisan config:cache && %[2]s artisan queue:restart; sudo -n /usr/bin/systemctl reload php%[3]s-fpm || true; }",
			current, php, s.PHPVersion))
		return err
	})
}
