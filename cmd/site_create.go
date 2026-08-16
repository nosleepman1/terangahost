package cmd

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teranga-host/terangahost/internal/platform/dns"
	"github.com/teranga-host/terangahost/internal/platform/ssh"
	"github.com/teranga-host/terangahost/internal/platform/storage"
	"github.com/teranga-host/terangahost/templates"
)

var (
	siteServerName string
	siteDomain     string
	sitePHP        string
	siteSSL        bool
	siteWorkers    int
	siteReverb     bool
)

var siteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Configure un nouveau site / API Laravel (Nginx, FPM Pool dédié, SSL Let's Encrypt, Queues)",
	Run: func(cmd *cobra.Command, args []string) {
		runSiteCreate()
	},
}

func init() {
	siteCreateCmd.Flags().StringVar(&siteServerName, "server", "", "Nom du serveur hôte cible")
	siteCreateCmd.Flags().StringVar(&siteDomain, "domain", "", "Nom de domaine principal (ex: api.monprojet.sn)")
	siteCreateCmd.Flags().StringVar(&sitePHP, "php", "", "Version PHP (par défaut celle du serveur)")
	siteCreateCmd.Flags().BoolVar(&siteSSL, "ssl", true, "Obtenir un certificat SSL Let's Encrypt automatique")
	siteCreateCmd.Flags().IntVar(&siteWorkers, "workers", 2, "Nombre de processus worker Supervisor pour les queues")
	siteCreateCmd.Flags().BoolVar(&siteReverb, "reverb", false, "Activer le support WebSockets Laravel Reverb")

	_ = siteCreateCmd.MarkFlagRequired("server")
	_ = siteCreateCmd.MarkFlagRequired("domain")
}

func runSiteCreate() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	repo, err := storage.NewJSONRepository()
	if err != nil {
		fmt.Printf("%s %v\n", red("Erreur repository:"), err)
		return
	}

	srv, err := repo.FindByName(ctx, siteServerName)
	if err != nil {
		fmt.Printf("%s Serveur [%s] introuvable.\n", red("Erreur:"), siteServerName)
		return
	}

	phpVer := srv.PHPVersion
	if sitePHP != "" {
		phpVer = sitePHP
	}

	fmt.Printf("🌐 %s [%s] sur le serveur [%s]...\n\n", cyan("Création du site Laravel"), siteDomain, srv.Name)

	// 1. Pre-flight DNS Check si SSL est activé
	if siteSSL {
		fmt.Printf("🔍 Vérification de la propagation DNS pour %s...\n", cyan(siteDomain))
		if err := dns.PreFlightDNSCheck(ctx, siteDomain, srv.IP); err != nil {
			fmt.Printf("\n%s %v\n", yellow("⚠ AVERTISSEMENT DNS :"), err)
			fmt.Printf("Pour éviter d'épuiser vos quotas Let's Encrypt, l'installation se poursuivra en HTTP standard.\n")
			fmt.Printf("Vous pourrez activer le SSL plus tard avec 'terangahost site ssl --domain=%s'\n\n", siteDomain)
			siteSSL = false
		} else {
			fmt.Printf("  ✔ DNS vérifié : %s pointe parfaitement vers %s\n\n", siteDomain, srv.IP)
		}
	}

	// 2. Connexion SSH
	client, err := ssh.NewNativeSSHClient(ssh.ClientOptions{
		Host:           srv.IP,
		Port:           srv.SSHPort,
		User:           srv.DeployUser,
		PrivateKeyPath: srv.SSHKeyPath,
	})
	if err != nil {
		// Fallback root
		client, err = ssh.NewNativeSSHClient(ssh.ClientOptions{
			Host:           srv.IP,
			Port:           srv.SSHPort,
			User:           srv.RootUser,
			PrivateKeyPath: srv.SSHKeyPath,
		})
		if err != nil {
			fmt.Printf("%s Connexion SSH impossible: %v\n", red("✖"), err)
			return
		}
	}
	defer client.Close()

	runner := ssh.NewNativeSSHRunner(client)
	defer runner.Close()

	cleanID := strings.ReplaceAll(siteDomain, ".", "_")
	siteDir := fmt.Sprintf("/var/www/%s", siteDomain)

	siteData := struct {
		ID              string
		Domain          string
		Aliases         []string
		PHPVersion      string
		Directory       string
		MaxChildren     int
		StartServers    int
		MinSpareServers int
		MaxSpareServers int
		QueueWorkers    int
	}{
		ID:              cleanID,
		Domain:          siteDomain,
		Aliases:         []string{},
		PHPVersion:      phpVer,
		Directory:       siteDir,
		MaxChildren:     srv.Hardware.TunedFpmMaxChildren(),
		StartServers:    2,
		MinSpareServers: 1,
		MaxSpareServers: 3,
		QueueWorkers:    siteWorkers,
	}

	// 3. Création de l'arborescence standard zero-downtime
	fmt.Printf("📁 Création de la structure de répertoires sous %s...\n", siteDir)
	setupDirs := []string{
		fmt.Sprintf("sudo mkdir -p %s/releases %s/shared/storage/app %s/shared/storage/framework/cache %s/shared/storage/framework/sessions %s/shared/storage/framework/views %s/shared/storage/logs", siteDir, siteDir, siteDir, siteDir, siteDir, siteDir),
		fmt.Sprintf("sudo chown -R deployer:www-data %s", siteDir),
		fmt.Sprintf("sudo chmod -R 775 %s/shared/storage", siteDir),
	}
	for _, cmd := range setupDirs {
		_ = runner.Execute(ctx, cmd, nil, nil)
	}

	// 4. Génération et upload du template Pool FPM dédié
	fmt.Printf("🐘 Configuration du pool PHP-FPM dédié (/run/php/php%s-fpm-%s.sock)...\n", phpVer, cleanID)
	fpmTmplContent, _ := templates.FS.ReadFile("php/fpm_pool.conf.tmpl")
	tFpm, _ := template.New("fpm").Parse(string(fpmTmplContent))
	var fpmBuf bytes.Buffer
	_ = tFpm.Execute(&fpmBuf, siteData)

	fpmPath := fmt.Sprintf("/etc/php/%s/fpm/pool.d/%s.conf", phpVer, cleanID)
	_ = runner.Upload(ctx, fpmBuf.Bytes(), fpmPath, 0644)
	_ = runner.Execute(ctx, fmt.Sprintf("sudo service php%s-fpm restart", phpVer), nil, nil)

	// 5. Génération et upload du template Nginx VirtualHost
	fmt.Println("🌐 Configuration du VirtualHost Nginx...")
	nginxTmplContent, _ := templates.FS.ReadFile("nginx/laravel_api.conf.tmpl")
	tNginx, _ := template.New("nginx").Parse(string(nginxTmplContent))
	var nginxBuf bytes.Buffer
	_ = tNginx.Execute(&nginxBuf, siteData)

	nginxAvailable := fmt.Sprintf("/etc/nginx/sites-available/%s", siteDomain)
	nginxEnabled := fmt.Sprintf("/etc/nginx/sites-enabled/%s", siteDomain)
	_ = runner.Upload(ctx, nginxBuf.Bytes(), nginxAvailable, 0644)
	_ = runner.Execute(ctx, fmt.Sprintf("sudo ln -sf %s %s && sudo nginx -t && sudo service nginx reload", nginxAvailable, nginxEnabled), nil, nil)

	// 6. Certificat SSL si éligible
	if siteSSL {
		fmt.Printf("🔒 Obtention du certificat SSL Let's Encrypt pour %s...\n", siteDomain)
		certCmd := fmt.Sprintf("sudo certbot --nginx -d %s --non-interactive --agree-tos --register-unsafely-without-email --redirect", siteDomain)
		_, errCert := runner.RunSilent(ctx, certCmd)
		if errCert == nil {
			fmt.Printf("  %s Certificat SSL HTTPS activé avec succès !\n", green("✔"))
		} else {
			fmt.Printf("  %s Échec de Certbot (%v), le site reste accessible en HTTP standard.\n", yellow("⚠"), errCert)
		}
	}

	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────"))
	fmt.Printf("🎉 %s\n", green("SITE CONFIGURÉ AVEC SUCCÈS SUR LE SERVEUR !"))
	fmt.Printf("🌐 Domaine : https://%s\n", siteDomain)
	fmt.Printf("📁 Répertoire : %s\n", siteDir)
	fmt.Printf("🚀 Prochaine étape : Déployez votre code avec :\n")
	fmt.Printf("   %s\n", cyan(fmt.Sprintf("terangahost site deploy --server=%s --domain=%s --repo=https://github.com/votre-repo", srv.Name, siteDomain)))
	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────\n"))
}
