package cmd

import (
	"fmt"
	"time"

	"github.com/nosleepman1/terangahost/internal/site"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var sdel struct {
	domain             string
	yes, purge, dropDB bool
}

var siteDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Retire un site du serveur (vhost, pool, workers, cron) ; fichiers et base conservés par défaut",
	RunE: func(*cobra.Command, []string) error {
		domainName, err := validate.Domain(sdel.domain)
		if err != nil {
			return err
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		ctx, cancel := commandContext(10 * time.Minute)
		defer cancel()
		s, srv, err := siteWithServer(ctx, store, domainName, "")
		if err != nil {
			return err
		}
		question := fmt.Sprintf("Désactiver %s sur %s ?", domainName, srv.Name)
		if sdel.purge {
			question += " Les fichiers de " + s.Directory + " et le certificat seront SUPPRIMÉS."
		}
		if sdel.dropDB && s.DBName != "" {
			question += " La base " + s.DBName + " sera SUPPRIMÉE."
		}
		if err := confirm(question, sdel.yes); err != nil {
			return err
		}
		r, err := connectAdmin(srv, openLog("delete_"+s.ID))
		if err != nil {
			return err
		}
		defer r.Close()
		m := &site.Manager{R: r, Srv: srv}
		if err := task("Suppression de la configuration de "+domainName, func() error {
			return m.Delete(ctx, s, site.DeleteOptions{PurgeFiles: sdel.purge, DropDatabase: sdel.dropDB})
		}); err != nil {
			return err
		}
		if err := store.DeleteSite(ctx, domainName); err != nil {
			return err
		}
		ui.Success("%s supprimé", domainName)
		if !sdel.purge {
			ui.Info("  Fichiers conservés dans %s (--purge pour les supprimer)", s.Directory)
		}
		return nil
	},
}

func init() {
	f := siteDeleteCmd.Flags()
	f.StringVar(&sdel.domain, "domain", "", "Domaine du site")
	f.BoolVarP(&sdel.yes, "yes", "y", false, "Ne pas demander de confirmation")
	f.BoolVar(&sdel.purge, "purge", false, "Supprime aussi les fichiers du site et son certificat")
	f.BoolVar(&sdel.dropDB, "drop-database", false, "Supprime aussi la base de données et son utilisateur")
	_ = siteDeleteCmd.MarkFlagRequired("domain")
	siteCmd.AddCommand(siteDeleteCmd)
}
