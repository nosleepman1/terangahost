package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/spf13/cobra"
)

var deployKeyServer string

var serverDeployKeyCmd = &cobra.Command{
	Use:   "deploy-key",
	Short: "Affiche la clé publique Git du deployer (à ajouter comme deploy key pour les dépôts privés)",
	RunE: func(*cobra.Command, []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		srv, err := store.FindServer(ctx, deployKeyServer)
		if err != nil {
			return err
		}
		r, err := connectDeployer(srv, nil)
		if err != nil {
			return err
		}
		defer r.Close()
		key, err := r.RunSilent(ctx, "cat ~/.ssh/id_ed25519.pub")
		if err != nil {
			return fmt.Errorf("clé introuvable (relancez 'terangahost server provision'): %w", err)
		}
		key = strings.TrimSpace(key)
		if srv.DeployKey != key {
			srv.DeployKey = key
			_ = store.SaveServer(ctx, srv)
		}
		fmt.Println(key)
		ui.Info("\n%s GitHub : Settings > Deploy keys > Add deploy key (lecture seule).", ui.Gray("→"))
		ui.Info("%s GitLab : Settings > Repository > Deploy keys.", ui.Gray("→"))
		return nil
	},
}

func init() {
	serverDeployKeyCmd.Flags().StringVar(&deployKeyServer, "name", "", "Nom du serveur")
	_ = serverDeployKeyCmd.MarkFlagRequired("name")
	serverCmd.AddCommand(serverDeployKeyCmd)
}
