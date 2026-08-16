package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teranga-host/terangahost/internal/platform/ssh"
	"github.com/teranga-host/terangahost/internal/platform/storage"
)

var (
	deployServerName string
	deployDomain     string
	deployRepo       string
	deployBranch     string
)

var siteDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Déploie une version de votre API Laravel en Zero-Downtime",
	Long: `Clone le repository Git dans un nouveau dossier de release, installe Composer,
applique les migrations, met en cache les routes et configs, puis bascule le symlink 'current'.`,
	Run: func(cmd *cobra.Command, args []string) {
		runSiteDeploy()
	},
}

func init() {
	siteDeployCmd.Flags().StringVar(&deployServerName, "server", "", "Nom du serveur hôte cible")
	siteDeployCmd.Flags().StringVar(&deployDomain, "domain", "", "Nom de domaine du site (ex: api.monprojet.sn)")
	siteDeployCmd.Flags().StringVar(&deployRepo, "repo", "", "URL du dépôt Git (HTTPS ou SSH)")
	siteDeployCmd.Flags().StringVar(&deployBranch, "branch", "main", "Branche Git à déployer (défaut: main)")

	_ = siteDeployCmd.MarkFlagRequired("server")
	_ = siteDeployCmd.MarkFlagRequired("domain")
	_ = siteDeployCmd.MarkFlagRequired("repo")
}

func runSiteDeploy() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	repo, err := storage.NewJSONRepository()
	if err != nil {
		fmt.Printf("%s %v\n", red("Erreur repository:"), err)
		return
	}

	srv, err := repo.FindByName(ctx, deployServerName)
	if err != nil {
		fmt.Printf("%s Serveur [%s] introuvable.\n", red("Erreur:"), deployServerName)
		return
	}

	fmt.Printf("🚀 %s pour [%s] sur [%s] (branche: %s)...\n\n",
		cyan("Démarrage du déploiement Zero-Downtime"), deployDomain, srv.Name, deployBranch)

	client, err := ssh.NewNativeSSHClient(ssh.ClientOptions{
		Host:           srv.IP,
		Port:           srv.SSHPort,
		User:           srv.DeployUser,
		PrivateKeyPath: srv.SSHKeyPath,
	})
	if err != nil {
		fmt.Printf("%s Connexion SSH impossible: %v\n", red("✖"), err)
		return
	}
	defer client.Close()

	runner := ssh.NewNativeSSHRunner(client)
	defer runner.Close()

	releaseID := time.Now().Format("20060102150405")
	siteDir := fmt.Sprintf("/var/www/%s", deployDomain)
	releaseDir := fmt.Sprintf("%s/releases/%s", siteDir, releaseID)

	deploySteps := []struct {
		desc string
		cmd  string
	}{
		{
			desc: "1. Clonage de la branche " + deployBranch,
			cmd:  fmt.Sprintf("git clone --depth 1 --branch %s %s %s", deployBranch, deployRepo, releaseDir),
		},
		{
			desc: "2. Liaison des fichiers et dossiers partagés (.env, storage/)",
			cmd: fmt.Sprintf("ln -nfs %s/shared/.env %s/.env && rm -rf %s/storage && ln -nfs %s/shared/storage %s/storage",
				siteDir, releaseDir, releaseDir, siteDir, releaseDir),
		},
		{
			desc: "3. Installation des dépendances Composer (Optimized Autoloader)",
			cmd:  fmt.Sprintf("cd %s && composer install --no-dev --no-interaction --prefer-dist --optimize-autoloader", releaseDir),
		},
		{
			desc: "4. Mise en cache des configurations, routes et vues Laravel",
			cmd:  fmt.Sprintf("cd %s && php artisan config:cache && php artisan route:cache && php artisan view:cache", releaseDir),
		},
		{
			desc: "5. Exécution des migrations de base de données (si configurée)",
			cmd:  fmt.Sprintf("cd %s && [ -f .env ] && php artisan migrate --force || true", releaseDir),
		},
		{
			desc: "6. Bascule atomique du lien symbolique (Zero-Downtime Switch)",
			cmd:  fmt.Sprintf("ln -sfn %s %s/current", releaseDir, siteDir),
		},
		{
			desc: "7. Rechargement de PHP-FPM et redémarrage des workers de queue",
			cmd: fmt.Sprintf("sudo service php%s-fpm reload && sudo supervisorctl restart all || true",
				srv.PHPVersion),
		},
		{
			desc: "8. Nettoyage des anciennes releases (conservation des 5 dernières)",
			cmd:  fmt.Sprintf("cd %s/releases && ls -t | tail -n +6 | xargs -r rm -rf", siteDir),
		},
	}

	for _, step := range deploySteps {
		fmt.Printf("  %s %s...\n", cyan("▶"), step.desc)
		if err := runner.Execute(ctx, step.cmd, nil, nil); err != nil {
			fmt.Printf("\n%s Échec à l'étape: %s\n%v\n", red("✖ ERREUR DÉPLOIEMENT :"), step.desc, err)
			return
		}
	}

	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────"))
	fmt.Printf("🎉 %s\n", green("DÉPLOIEMENT ZERO-DOWNTIME TERMINÉ AVEC SUCCÈS !"))
	fmt.Printf("🌐 API en ligne : https://%s\n", deployDomain)
	fmt.Printf("📦 Release ID : %s\n", releaseID)
	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────\n"))
}
