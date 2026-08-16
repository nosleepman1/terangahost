package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/teranga-host/terangahost/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "terangahost",
	Short: "TerangaHost - Provisionneur et gestionnaire d'infrastructure VPS pour APIs Laravel",
	Long: `TerangaHost est un outil CLI Open Source concu pour automatiser la configuration
et le deploiement securise d'applications et d'APIs Laravel sur VPS Ubuntu.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Name() != "help" && cmd.Name() != "version" {
			ui.PrintBanner()
		}
	},
}

// Execute lance la commande racine
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Erreur: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(siteCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Affiche la version de TerangaHost",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TerangaHost v1.0.0 (Go runtime)")
	},
}
