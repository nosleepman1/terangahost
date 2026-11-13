package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/spf13/cobra"
)

var siteListServer string

var siteListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les sites enregistrés",
	RunE: func(*cobra.Command, []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		ctx := context.Background()
		serverID := ""
		if siteListServer != "" {
			srv, err := store.FindServer(ctx, siteListServer)
			if err != nil {
				return err
			}
			serverID = srv.ID
		}
		sites, err := store.ListSites(ctx, serverID)
		if err != nil {
			return err
		}
		if len(sites) == 0 {
			ui.Info("Aucun site enregistré.")
			return nil
		}
		servers, err := store.ListServers(ctx)
		if err != nil {
			return err
		}
		names := map[string]string{}
		for _, s := range servers {
			names[s.ID] = s.Name
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DOMAINE\tSERVEUR\tPHP\tSSL\tWORKERS\tRELEASE\tCOMMIT\tDÉPLOYÉ LE")
		for _, s := range sites {
			deployed := "-"
			if !s.LastDeployAt.IsZero() {
				deployed = s.LastDeployAt.Local().Format("02/01/2006 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%d\t%s\t%s\t%s\n", s.Domain, names[s.ServerID], s.PHPVersion, s.HasSSL, s.QueueWorkers,
				firstNonEmpty(s.CurrentRelease, "-"), firstNonEmpty(s.LastCommit, "-"), deployed)
		}
		return w.Flush()
	},
}

func init() {
	siteListCmd.Flags().StringVar(&siteListServer, "server", "", "Filtre sur un serveur")
	siteCmd.AddCommand(siteListCmd)
}
