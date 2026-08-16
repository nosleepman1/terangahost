package cmd

import "github.com/spf13/cobra"

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Gestion et provisionnement des serveurs VPS",
	Long:  "Commandes pour provisionner, diagnostiquer et lister vos serveurs VPS TerangaHost.",
}

func init() {
	serverCmd.AddCommand(serverProvisionCmd)
	serverCmd.AddCommand(serverDoctorCmd)
	serverCmd.AddCommand(serverListCmd)
}
