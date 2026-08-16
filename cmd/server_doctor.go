package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teranga-host/terangahost/internal/platform/ssh"
	"github.com/teranga-host/terangahost/internal/platform/storage"
)

var doctorServerName string

var serverDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnostique la santé globale du serveur VPS (RAM, Disque, Services Nginx/PHP/DB)",
	Long:  "Inspecte l'état des services, l'espace disque restant, la mémoire et les erreurs récentes.",
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor()
	},
}

func init() {
	serverDoctorCmd.Flags().StringVar(&doctorServerName, "name", "", "Nom du serveur à inspecter")
	_ = serverDoctorCmd.MarkFlagRequired("name")
}

func runDoctor() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repo, err := storage.NewJSONRepository()
	if err != nil {
		fmt.Printf("%s %v\n", red("Erreur repository:"), err)
		return
	}

	srv, err := repo.FindByName(ctx, doctorServerName)
	if err != nil {
		fmt.Printf("%s Serveur [%s] introuvable dans la base locale.\n", red("Erreur:"), doctorServerName)
		return
	}

	fmt.Printf("🩺 %s [%s] (%s)...\n\n", cyan("Diagnostic de santé pour le serveur"), srv.Name, srv.IP)

	client, err := ssh.NewNativeSSHClient(ssh.ClientOptions{
		Host:           srv.IP,
		Port:           srv.SSHPort,
		User:           srv.DeployUser,
		PrivateKeyPath: srv.SSHKeyPath,
		Timeout:        10 * time.Second,
	})
	if err != nil {
		// Repli sur l'utilisateur root si deployer n'est pas accessible
		client, err = ssh.NewNativeSSHClient(ssh.ClientOptions{
			Host:           srv.IP,
			Port:           srv.SSHPort,
			User:           srv.RootUser,
			PrivateKeyPath: srv.SSHKeyPath,
			Timeout:        10 * time.Second,
		})
		if err != nil {
			fmt.Printf("%s Connexion SSH impossible: %v\n", red("✖"), err)
			return
		}
	}
	defer client.Close()

	runner := ssh.NewNativeSSHRunner(client)
	defer runner.Close()

	// 1. Diagnostic des Services
	fmt.Println(cyan("  [1/3] Statut des Services Système :"))
	services := []string{"nginx", fmt.Sprintf("php%s-fpm", srv.PHPVersion), "supervisor"}
	if srv.Database == "mariadb" || srv.Database == "mysql" {
		services = append(services, "mariadb")
	}
	if srv.WithRedis {
		services = append(services, "redis-server")
	}

	for _, svc := range services {
		cmd := fmt.Sprintf("systemctl is-active %s", svc)
		out, err := runner.RunSilent(ctx, cmd)
		if err == nil && strings.TrimSpace(out) == "active" {
			fmt.Printf("    ✔ Service %-15s : %s\n", svc, green("ACTIF (En cours d'exécution)"))
		} else {
			fmt.Printf("    ✖ Service %-15s : %s\n", svc, red("INACTIF / EN ERREUR"))
		}
	}

	// 2. Diagnostic des Ressources
	fmt.Printf("\n%s\n", cyan("  [2/3] État des Ressources Système :"))
	diskCmd := "df -h / | awk 'NR==2 {print $4, \"disponible sur\", $2, \"(\"$5, \"utilisé)\"}'"
	diskOut, _ := runner.RunSilent(ctx, diskCmd)
	fmt.Printf("    💾 Disque : %s\n", diskOut)

	ramCmd := "free -h | awk '/^Mem:/{print $4, \"libre sur\", $2}'"
	ramOut, _ := runner.RunSilent(ctx, ramCmd)
	fmt.Printf("    🧠 RAM    : %s\n", ramOut)

	swapCmd := "free -h | awk '/^Swap:/{print $3, \"utilisé sur\", $2}'"
	swapOut, _ := runner.RunSilent(ctx, swapCmd)
	fmt.Printf("    🔄 Swap   : %s\n", swapOut)

	// 3. Diagnostic des Logs d'erreurs récents
	fmt.Printf("\n%s\n", cyan("  [3/3] Recherche d'erreurs Nginx récentes :"))
	logCmd := "tail -n 5 /var/log/nginx/error.log 2>/dev/null || echo 'Aucun log disponible'"
	logOut, _ := runner.RunSilent(ctx, logCmd)
	if strings.TrimSpace(logOut) == "" || strings.Contains(logOut, "Aucun log") {
		fmt.Printf("    ✔ %s\n", green("Aucune erreur critique récente dans Nginx"))
	} else {
		fmt.Printf("    %s\n%s\n", yellow("Dernières traces :"), color.HiBlackString(logOut))
	}

	fmt.Println()
}
