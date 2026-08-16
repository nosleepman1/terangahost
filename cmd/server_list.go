package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/teranga-host/terangahost/internal/platform/storage"
)

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste tous les serveurs VPS enregistres et geres par TerangaHost",
	Run: func(cmd *cobra.Command, args []string) {
		runList()
	},
}

func runList() {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	green := color.New(color.FgGreen, color.Bold).SprintFunc()
	gold := color.New(color.FgHiYellow, color.Bold).SprintFunc()

	repo, err := storage.NewJSONRepository()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erreur repository: %v\n", err)
		return
	}

	servers, err := repo.List(context.Background())
	if err != nil || len(servers) == 0 {
		fmt.Println("Aucun serveur enregistre. Utilisez :")
		fmt.Printf("  %s\n\n", cyan("terangahost server provision --name=mon-serveur --ip=..."))
		return
	}

	fmt.Printf("%s (%d enregistres)\n\n", gold("Serveurs TerangaHost"), len(servers))
	fmt.Printf("  %-18s %-16s %-10s %-8s %-12s %-10s\n", "NOM", "IP", "USER", "PHP", "DATABASE", "STATUS")
	fmt.Println(color.HiBlackString("  ──────────────────────────────────────────────────────────────────────────"))

	for _, s := range servers {
		statusFormatted := green(s.Status)
		if s.Status != "ready" {
			statusFormatted = color.YellowString(s.Status)
		}

		fmt.Printf("  %-18s %-16s %-10s %-8s %-12s %-10s\n",
			cyan(s.Name),
			s.IP,
			s.DeployUser,
			s.PHPVersion,
			s.Database,
			statusFormatted,
		)
	}
	fmt.Println()
}
