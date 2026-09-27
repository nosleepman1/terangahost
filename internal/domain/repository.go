package domain

import "context"

// Store définit le contrat de persistance locale des serveurs et des sites.
type Store interface {
	SaveServer(ctx context.Context, server *Server) error
	FindServer(ctx context.Context, name string) (*Server, error)
	FindServerByID(ctx context.Context, id string) (*Server, error)
	ListServers(ctx context.Context) ([]*Server, error)
	// DeleteServer supprime le serveur et tous ses sites de la configuration locale.
	DeleteServer(ctx context.Context, name string) error

	SaveSite(ctx context.Context, site *Site) error
	FindSite(ctx context.Context, domain string) (*Site, error)
	// ListSites retourne les sites d'un serveur, ou tous les sites si serverID est vide.
	ListSites(ctx context.Context, serverID string) ([]*Site, error)
	DeleteSite(ctx context.Context, domain string) error
}
