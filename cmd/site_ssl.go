package cmd

import (
	"time"

	"github.com/nosleepman1/terangahost/internal/site"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/nosleepman1/terangahost/internal/validate"
	"github.com/spf13/cobra"
)

var ss struct {
	domain, email string
	skipDNS       bool
}

var siteSSLCmd = &cobra.Command{
	Use:   "ssl",
	Short: "Active HTTPS (Let's Encrypt) sur un site existant, renouvellement automatique inclus",
	RunE: func(*cobra.Command, []string) error {
		domainName, err := validate.Domain(ss.domain)
		if err != nil {
			return err
		}
		if ss.email != "" {
			if err := validate.Email(ss.email); err != nil {
				return err
			}
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
		log := openLog("ssl_" + s.ID)
		defer log.Close()
		r, err := connectAdmin(srv, log)
		if err != nil {
			return err
		}
		defer r.Close()
		m := &site.Manager{R: r, Srv: srv}
		if err := enableSSL(ctx, m, srv, s, ss.email, ss.skipDNS); err != nil {
			return err
		}
		s.HasSSL = true
		if err := store.SaveSite(ctx, s); err != nil {
			return err
		}
		ui.Success("HTTPS actif : https://%s (renouvellement automatique par certbot)", domainName)
		return nil
	},
}

func init() {
	siteSSLCmd.Flags().StringVar(&ss.domain, "domain", "", "Domaine du site")
	siteSSLCmd.Flags().StringVar(&ss.email, "email", "", "E-mail Let's Encrypt (recommandé)")
	siteSSLCmd.Flags().BoolVar(&ss.skipDNS, "skip-dns-check", false, "Ignore la vérification DNS préalable")
	_ = siteSSLCmd.MarkFlagRequired("domain")
	siteCmd.AddCommand(siteSSLCmd)
}
