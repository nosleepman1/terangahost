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
	Long: `Configure un serveur Ubuntu 22.04/24.04: pare-feu UFW, Fail2ban, utilisateur deployer,
swap 2GB, PHP 8.x (14 extensions), Nginx, Supervisor, Composer, Certbot et Base de donnees.`,
	Run: func(cmd *cobra.Command, args []string) {
		runProvision()
	},
}

func init() {
	serverProvisionCmd.Flags().StringVar(&provName, "name", "", "Nom unique du serveur (ex: dakar-prod)")
	serverProvisionCmd.Flags().StringVar(&provIP, "ip", "", "Adresse IP publique du VPS (ex: 192.168.1.50)")
	serverProvisionCmd.Flags().IntVar(&provPort, "port", 22, "Port SSH (defaut: 22)")
	serverProvisionCmd.Flags().StringVar(&provUser, "user", "root", "Utilisateur initial pour la connexion SSH")
	serverProvisionCmd.Flags().StringVar(&provKey, "ssh-key", "", "Chemin vers votre cle privee SSH (ex: ~/.ssh/id_ed25519)")
	serverProvisionCmd.Flags().StringVar(&provPassword, "password", "", "Mot de passe SSH (optionnel si cle utilisee)")
	serverProvisionCmd.Flags().StringVar(&provPHP, "php", "8.3", "Version de PHP a installer (8.2, 8.3, 8.4)")
	serverProvisionCmd.Flags().StringVar(&provDatabase, "db", "mariadb", "Base de donnees ('mariadb', 'postgres', 'none')")
	serverProvisionCmd.Flags().BoolVar(&provWithRedis, "redis", true, "Installer et activer le serveur Redis")

	_ = serverProvisionCmd.MarkFlagRequired("ip")
	_ = serverProvisionCmd.MarkFlagRequired("name")
}

func runProvision() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()

	fmt.Printf("[INFO] Initialisation du provisionnement pour le serveur [%s] (%s)...\n\n", provName, provIP)

	// 1. Initialisation du Context et gestion de l'interruption (Ctrl+C)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n%s Interruption demandee. Arret securise du pipeline...\n", yellow("[SIGNAL: INTERRUPT]"))
		cancel()
	}()

	// 2. Initialisation du logger sur disque
	logFile, logPath, err := logger.NewFileLogger("provision_" + provName)
	if err == nil {
		defer logFile.Close()
		fmt.Printf("[LOG] Fichier journal detaille: %s\n\n", color.HiBlackString(logPath))
	}

	// 3. Connexion SSH
	fmt.Printf("[SSH] Connexion vers %s@%s:%d...\n", provUser, provIP, provPort)
	client, err := ssh.NewNativeSSHClient(ssh.ClientOptions{
		Host:           provIP,
		Port:           provPort,
		User:           provUser,
		PrivateKeyPath: provKey,
		Password:       provPassword,
		Timeout:        20 * time.Second,
	})
	if err != nil {
		fmt.Printf("\n%s Impossible d'etablir la connexion SSH: %v\n", red("[ERROR]"), err)
		return
	}
	defer client.Close()

	runner := ssh.NewNativeSSHRunner(client)
	defer runner.Close()

	// 4. Detection du materiel (Hardware-Aware)
	fmt.Printf("[HARDWARE] Analyse des specifications du serveur cible...\n")
	spec, err := engine.DetectHardware(ctx, runner)
	if err != nil {
		fmt.Printf("  %s %v (valeurs par defaut appliquees)\n", yellow("[WARN]"), err)
		spec.TotalRAMMB = 1024
		spec.CPUCores = 1
	} else {
		fmt.Printf("  [HARDWARE] Detecte: %s | %d MB RAM | %d vCPU | %d GB Disque libre\n\n",
			cyan(spec.OSVersion), spec.TotalRAMMB, spec.CPUCores, spec.DiskFreeGB)
	}

	// 5. Creation de l'entite Server
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

	// 6. Construction et execution du Pipeline
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
		fmt.Printf("\n%s Le provisionnement a echoue: %v\n", red("[FATAL ERROR]"), err)
		fmt.Printf("Consultez le fichier journal: %s\n", logPath)
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
	fmt.Printf("%s Serveur provisionne avec succes pour Laravel.\n", green("[SUCCESS]"))
	fmt.Printf("  - Duree d'execution: %s\n", cyan(totalDuration.String()))
	fmt.Printf("  - Utilisateur applicatif: %s\n", green("deployer"))
	fmt.Printf("  - Serveur HTTP: %s\n", green("Nginx"))
	fmt.Printf("  - Moteur PHP: %s (14 extensions + OPcache JIT)\n", green("PHP "+provPHP))
	fmt.Printf("  - Base de donnees: %s\n", green(provDatabase))
	fmt.Println(color.HiBlackString("──────────────────────────────────────────────────────────────────────────"))
	fmt.Println("\nConfiguration d'une API sur cette instance :")
	fmt.Printf("  %s\n\n", cyan("terangahost site create --server="+provName+" --domain=api.domaine.com"))
}
