package cmd

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/nosleepman1/terangahost/internal/platform/ssh"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/spf13/cobra"
)

var (
	removeName string
	removeYes  bool
	forgetHost string
)

var serverRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Retire un serveur (et ses sites) de l'inventaire local, sans toucher au VPS",
	RunE: func(*cobra.Command, []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		ctx := context.Background()
		srv, err := store.FindServer(ctx, removeName)
		if err != nil {
			return err
		}
		if err := confirm(fmt.Sprintf("Retirer %s (%s) de l'inventaire local ?", srv.Name, srv.IP), removeYes); err != nil {
			return err
		}
		if err := store.DeleteServer(ctx, srv.Name); err != nil {
			return err
		}
		_, _ = ssh.ForgetHost(net.JoinHostPort(srv.IP, strconv.Itoa(srv.SSHPort)))
		ui.Success("%s retiré de l'inventaire local (le VPS n'a pas été modifié)", srv.Name)
		return nil
	},
}

var serverForgetHostCmd = &cobra.Command{
	Use:   "forget-host",
	Short: "Oublie l'empreinte SSH mémorisée d'un hôte (après réinstallation du VPS)",
	RunE: func(*cobra.Command, []string) error {
		host := forgetHost
		if _, _, err := net.SplitHostPort(host); err != nil {
			host = net.JoinHostPort(host, "22")
		}
		n, err := ssh.ForgetHost(host)
		if err != nil {
			return err
		}
		if n == 0 {
			ui.Warn("aucune empreinte mémorisée pour %s", host)
			return nil
		}
		ui.Success("empreinte de %s oubliée : elle sera mémorisée à la prochaine connexion", host)
		return nil
	},
}

func init() {
	serverRemoveCmd.Flags().StringVar(&removeName, "name", "", "Nom du serveur")
	serverRemoveCmd.Flags().BoolVarP(&removeYes, "yes", "y", false, "Ne pas demander de confirmation")
	_ = serverRemoveCmd.MarkFlagRequired("name")
	serverForgetHostCmd.Flags().StringVar(&forgetHost, "host", "", "Adresse de l'hôte (ip ou ip:port)")
	_ = serverForgetHostCmd.MarkFlagRequired("host")
	serverCmd.AddCommand(serverRemoveCmd, serverForgetHostCmd)
}
