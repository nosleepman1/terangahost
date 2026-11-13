package cmd

import "github.com/spf13/cobra"

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Provisionnement, diagnostic et inventaire des serveurs",
}
