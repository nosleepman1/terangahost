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
	Short: "Diagnostique la sante globale du serveur VPS (RAM, Disque, Services Nginx/PHP/DB)",
	Long:  "Inspecte l'etat des services, l'espace disque disponible, la memoire et les erreurs recentes.",
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor()
	},
}

func init() {
	serverDoctorCmd.Flags().StringVar(&doctorServerName, "name", "", "Nom du serveur a inspecter")
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
		fmt.Printf("%s %v\n", red("[ERROR] Repository:"), err)
		return
	}

	srv, err := repo.FindByName(ctx, doctorServerName)
	if err != nil {
		fmt.Printf("%s Serveur [%s] introuvable dans la configuration locale.\n", red("[ERROR]"), doctorServerName)
		return
	}

	fmt.Printf("[DOCTOR] Analyse du serveur [%s] (%s)...\n\n", srv.Name, srv.IP)

	client, err := ssh.NewNativeSSHClient(ssh.ClientOptions{
		Host:           srv.IP,
		Port:           srv.SSHPort,
		User:           srv.DeployUser,
		PrivateKeyPath: srv.SSHKeyPath,
		Timeout:        10 * time.Second,
	})
	if err != nil {
		client, err = ssh.NewNativeSSHClient(ssh.ClientOptions{
			Host:           srv.IP,
			Port:           srv.SSHPort,
			User:           srv.RootUser,
			PrivateKeyPath: srv.SSHKeyPath,
			Timeout:        10 * time.Second,
		})
		if err != nil {
			fmt.Printf("%s Echec de connexion SSH: %v\n", red("[ERROR]"), err)
			return
		}
	}
	defer client.Close()

	runner := ssh.NewNativeSSHRunner(client)
	defer runner.Close()

	// 1. Diagnostic des Services
	fmt.Println(cyan("  [1/3] Statut des Services Systeme :"))
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
			fmt.Printf("    [OK] Service %-15s : %s\n", svc, green("ACTIF"))
		} else {
			fmt.Printf("    [FAIL] Service %-15s : %s\n", svc, red("INACTIF / ERREUR"))
		}
	}

	// 2. Diagnostic des Ressources
	fmt.Printf("\n%s\n", cyan("  [2/3] Etat des Ressources :"))
	diskCmd := "df -h / | awk 'NR==2 {print $4, \"disponible sur\", $2, \"(\"$5, \"utilise)\"}'"
	diskOut, _ := runner.RunSilent(ctx, diskCmd)
	fmt.Printf("    Disque : %s\n", diskOut)

	ramCmd := "free -h | awk '/^Mem:/{print $4, \"libre sur\", $2}'"
	ramOut, _ := runner.RunSilent(ctx, ramCmd)
	fmt.Printf("    RAM    : %s\n", ramOut)

	swapCmd := "free -h | awk '/^Swap:/{print $3, \"utilise sur\", $2}'"
	swapOut, _ := runner.RunSilent(ctx, swapCmd)
	fmt.Printf("    Swap   : %s\n", swapOut)

	// 3. Diagnostic des Logs d'erreurs récents
	fmt.Printf("\n%s\n", cyan("  [3/3] Journal d'erreurs Nginx :"))
	logCmd := "tail -n 5 /var/log/nginx/error.log 2>/dev/null || echo 'Aucun log'"
	logOut, _ := runner.RunSilent(ctx, logCmd)
	if strings.TrimSpace(logOut) == "" || strings.Contains(logOut, "Aucun log") {
		fmt.Printf("    [OK] %s\n", green("Aucune erreur critique recente dans Nginx"))
	} else {
		fmt.Printf("    %s\n%s\n", yellow("[WARN] Dernieres traces :"), color.HiBlackString(logOut))
	}

	fmt.Println()
}
