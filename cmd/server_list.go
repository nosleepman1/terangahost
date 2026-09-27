package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/spf13/cobra"
)

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les serveurs enregistrés",
	RunE: func(*cobra.Command, []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		ctx := context.Background()
		servers, err := store.ListServers(ctx)
		if err != nil {
			return err
		}
		if len(servers) == 0 {
			ui.Info("Aucun serveur enregistré. Pour commencer :")
			ui.Hint("terangahost server provision --name=prod --ip=203.0.113.10")
			return nil
		}
		sites, err := store.ListSites(ctx, "")
		if err != nil {
			return err
		}
		count := map[string]int{}
		for _, s := range sites {
			count[s.ServerID]++
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NOM\tIP\tADMIN\tPHP\tBASE\tREDIS\tSITES\tSTATUT")
		for _, s := range servers {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%v\t%d\t%s\n", s.Name, s.IP, s.AdminUser, s.PHPVersion, s.Database, s.WithRedis, count[s.ID], status(s.Status))
		}
		return w.Flush()
	},
}

func status(s string) string {
	switch s {
	case domain.StatusReady:
		return "prêt"
	case domain.StatusError:
		return "erreur"
	case domain.StatusProvisioning:
		return "incomplet"
	}
	return s
}

func init() {
	serverCmd.AddCommand(serverListCmd)
}
