// Package cmd définit l'interface en ligne de commande de TerangaHost.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/spf13/cobra"
)

// Version est injectée à la compilation : -ldflags "-X github.com/nosleepman1/terangahost/cmd.Version=v1.2.3".
var Version = "dev"

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "terangahost",
	Short: "Provisionnement de VPS Ubuntu et déploiement zero-downtime d'applications Laravel",
	Long: `TerangaHost configure un VPS Ubuntu 22.04/24.04 pour Laravel (Nginx, PHP-FPM, base de données,
Redis, Supervisor, SSL) puis déploie vos applications sans interruption de service.

Démarrage rapide :
  terangahost server provision --name=prod --ip=203.0.113.10 --ssh-key=~/.ssh/id_ed25519
  terangahost site create --server=prod --domain=api.exemple.com --repo=git@github.com:org/api.git
  terangahost site deploy --domain=api.exemple.com`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute lance la CLI et retourne le code de sortie du processus.
func Execute() int {
	rootCmd.Version = Version
	if err := rootCmd.Execute(); err != nil {
		printError(err)
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Affiche la sortie détaillée des commandes distantes")
	rootCmd.PersistentPreRun = func(*cobra.Command, []string) {
		if verbose {
			ui.SpinnerEnabled = false
		}
	}
	rootCmd.AddCommand(serverCmd, siteCmd, versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Affiche la version de TerangaHost",
	Run: func(*cobra.Command, []string) {
		fmt.Printf("TerangaHost %s\n", Version)
	},
}

// printError affiche l'erreur et, lorsque c'est possible, la marche à suivre.
func printError(err error) {
	fmt.Fprintf(os.Stderr, "\n%s %v\n", ui.Red("Erreur :"), err)
	hint := ""
	switch {
	case errors.Is(err, domain.ErrSSHAuthentication):
		hint = "Précisez la clé avec --ssh-key, chargez-la dans ssh-agent, ou utilisez --ask-password."
	case errors.Is(err, domain.ErrSSHConnectionTimeout):
		hint = "Vérifiez l'adresse IP, le port SSH (--port) et le pare-feu de l'hébergeur."
	case errors.Is(err, domain.ErrHostKeyMismatch):
		hint = "Si le VPS a été réinstallé : terangahost server forget-host --host <ip>"
	case errors.Is(err, domain.ErrSudoRequired):
		hint = "Connectez-vous en root (--user=root) ou configurez sudo sans mot de passe pour cet utilisateur."
	case errors.Is(err, domain.ErrServerNotFound):
		hint = "Listez les serveurs connus : terangahost server list"
	case errors.Is(err, domain.ErrSiteNotFound):
		hint = "Listez les sites connus : terangahost site list"
	case errors.Is(err, domain.ErrDNSPropagationPending):
		hint = "Créez l'enregistrement DNS A vers l'IP du VPS, attendez la propagation puis : terangahost site ssl --domain <domaine>"
	case errors.Is(err, domain.ErrConfigCorrupted):
		hint = "Le fichier n'a pas été modifié. Restaurez config.json.bak s'il existe, ou corrigez le JSON."
	}
	if hint != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", ui.Yellow("Conseil :"), hint)
	}
}
