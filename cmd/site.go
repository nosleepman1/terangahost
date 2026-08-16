package cmd

import "github.com/spf13/cobra"

var siteCmd = &cobra.Command{
	Use:   "site",
	Short: "Création et gestion des applications / APIs Laravel",
	Long:  "Commandes pour configurer des VirtualHosts Nginx, isoler les pools PHP-FPM, et déployer des APIs Laravel.",
}

func init() {
	siteCmd.AddCommand(siteCreateCmd)
	siteCmd.AddCommand(siteDeployCmd)
}
