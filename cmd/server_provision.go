package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teranga-host/terangahost/internal/domain"
	"github.com/teranga-host/terangahost/internal/engine"
	"github.com/teranga-host/terangahost/internal/engine/steps"
	"github.com/teranga-host/terangahost/internal/platform/logger"
	"github.com/teranga-host/terangahost/internal/platform/ssh"
	"github.com/teranga-host/terangahost/internal/platform/storage"
)

var (
	provName       string
	provIP         string
	provPort       int
	provUser       string
	provKey        string
	provPassword   string
	provPHP        string
	provDatabase   string
	provWithRedis  bool
)

var serverProvisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provisionne un serveur VPS Ubuntu vierge en environnement Laravel de production",
	Long: `Prend un VPS Ubuntu 22.04/24.04 vierge et automatise la sécurité, la création de swap,
l'installation de PHP 8.x (14 extensions), Nginx, Supervisor, Composer, Certbot et Base de Données.`,
	Run: func(cmd *cobra.Command, args []string) {
		runProvision()
	},
}

func init() {
	serverProvisionCmd.Flags().StringVar(&provName, "name", "", "Nom unique du serveur (ex: dakar-prod)")
	serverProvisionCmd.Flags().StringVar(&provIP, "ip", "", "Adresse IP publique du VPS (ex: 192.168.1.50)")
	serverProvisionCmd.Flags().IntVar(&provPort, "port", 22, "Port SSH (défaut: 22)")
	serverProvisionCmd.Flags().StringVar(&provUser, "user", "root", "Utilisateur initial pour la connexion SSH")
	serverProvisionCmd.Flags().StringVar(&provKey, "ssh-key", "", "Chemin vers votre clé privée SSH (ex: ~/.ssh/id_ed25519)")
	serverProvisionCmd.Flags().StringVar(&provPassword, "password", "", "Mot de passe SSH (optionnel si clé utilisée)")
	serverProvisionCmd.Flags().StringVar(&provPHP, "php", "8.3", "Version de PHP à installer (8.2, 8.3, 8.4)")
	serverProvisionCmd.Flags().StringVar(&provDatabase, "db", "mariadb", "Base de données ('mariadb', 'postgres', 'none')")
	serverProvisionCmd.Flags().BoolVar(&provWithRedis, "redis", true, "Installer et activer le serveur Redis")

	_ = serverProvisionCmd.MarkFlagRequired("ip")
	_ = serverProvisionCmd.MarkFlagRequired("name")
}

func runProvision() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()

	fmt.Printf("🚀 %s pour le serveur [%s] (%s)...\n\n", cyan("Démarrage du provisionnement TerangaHost"), provName, provIP)

	// 1. Initialisation du Context et gestion de l'interruption (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n%s Interruption demandée. Arrêt sécurisé du pipeline...\n", yellow("⚠ [CTRL+C]"))
		cancel()
	}()

	// 2. Initialisation du logger sur disque
	logFile, logPath, err := logger.NewFileLogger("provision_" + provName)
	if err == nil {
		defer logFile.Close()
		fmt.Printf("📝 Fichier de logs détaillé : %s\n\n", color.HiBlackString(logPath))
	}

	// 3. Connexion SSH
	fmt.Printf("🔌 Connexion SSH vers %s@%s:%d...\n", provUser, provIP, provPort)
	client, err := ssh.NewNativeSSHClient(ssh.ClientOptions{
		Host:           provIP,
		Port:           provPort,
		User:           provUser,
		PrivateKeyPath: provKey,
		Password:       provPassword,
		Timeout:        20 * time.Second,
	})
	if err != nil {
		fmt.Printf("\n%s Impossible de se connecter en SSH: %v\n", red("✖ ERREUR:"), err)
		return
	}
	defer client.Close()

	runner := ssh.NewNativeSSHRunner(client)
	defer runner.Close()

	// 4. Détection du matériel (Hardware-Aware)
	fmt.Printf("🔍 Analyse des spécifications matérielles du VPS...\n")
	spec, err := engine.DetectHardware(ctx, runner)
	if err != nil {
		fmt.Printf("  %s %v (continuation avec valeurs par défaut)\n", yellow("⚠"), err)
		spec.TotalRAMMB = 1024
		spec.CPUCores = 1
	} else {
		fmt.Printf("  ⚡ Détecté: %s | %d Mo RAM | %d vCPU | %d Go Disque libre\n\n",
			cyan(spec.OSVersion), spec.TotalRAMMB, spec.CPUCores, spec.DiskFreeGB)
	}

	// 5. Création de l'entité Server
	srv := &domain.Server{
		ID:         fmt.Sprintf("srv_%d", time.Now().Unix()),
		Name:       provName,
		IP:         provIP,
		SSHPort:    provPort,
		RootUser:   provUser,
		DeployUser: "deployer",
		SSHKeyPath: provKey,
		PHPVersion: provPHP,
		Database:   provDatabase,
		WithRedis:  provWithRedis,
		Hardware:   spec,
		Status:     "provisioning",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 6. Construction et exécution du Pipeline
	listener := &engine.DefaultConsoleListener{Writer: os.Stdout}
	pipeline := engine.NewPipeline(listener)

	pipeline.AddStep(&steps.StepHandshake{})
	pipeline.AddStep(&steps.StepSwap{})
	pipeline.AddStep(&steps.StepSecurity{})
	pipeline.AddStep(&steps.StepSudoers{})
	pipeline.AddStep(&steps.StepPHP{})
	pipeline.AddStep(&steps.StepWebServer{})
	pipeline.AddStep(&steps.StepTools{})
	pipeline.AddStep(&steps.StepDatabase{})

	pipelineStart := time.Now()
	if err := pipeline.Execute(ctx, runner, srv, logFile); err != nil {
		fmt.Printf("\n%s Le provisionnement a échoué: %v\n", red("✖ ERREUR FATALE:"), err)
		fmt.Printf("Consultez le fichier log pour plus de détails: %s\n", logPath)
		return
	}

	srv.Status = "ready"

	// 7. Sauvegarde dans le repo local
	repo, err := storage.NewJSONRepository()
	if err == nil {
		_ = repo.Save(ctx, srv)
	}

	totalDuration := time.Since(pipelineStart).Round(time.Second)

	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────"))
	fmt.Printf("🎉 %s\n", green("SERVEUR PROVISIONNÉ AVEC SUCCÈS POUR LARAVEL !"))
	fmt.Printf("⏱  Temps total d'exécution : %s\n", cyan(totalDuration.String()))
	fmt.Printf("👤 Utilisateur déployeur créé : %s\n", green("deployer"))
	fmt.Printf("🌐 Serveur Web : %s\n", green("Nginx (Optimisé API + WebSockets)"))
	fmt.Printf("🐘 Version PHP : %s (14 extensions + OPcache JIT)\n", green("PHP "+provPHP))
	fmt.Printf("💾 Base de données : %s\n", green(provDatabase))
	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────"))
	fmt.Println("\n👉 Pour créer et déployer une API Laravel sur ce serveur :")
	fmt.Printf("   %s\n\n", cyan("terangahost site create --server="+provName+" --domain=api.monprojet.sn"))
}
