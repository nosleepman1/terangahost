package domain

import "context"

// ServerRepository définit le contrat de persistance locale des serveurs
type ServerRepository interface {
	Save(ctx context.Context, server *Server) error
	FindByID(ctx context.Context, id string) (*Server, error)
	FindByName(ctx context.Context, name string) (*Server, error)
	FindByIP(ctx context.Context, ip string) (*Server, error)
	List(ctx context.Context) ([]*Server, error)
	Delete(ctx context.Context, id string) error
}

// SiteRepository définit le contrat de persistance locale des applications/sites
type SiteRepository interface {
	Save(ctx context.Context, site *Site) error
	FindByID(ctx context.Context, id string) (*Site, error)
	FindByDomain(ctx context.Context, domain string) (*Site, error)
	ListByServer(ctx context.Context, serverID string) ([]*Site, error)
	Delete(ctx context.Context, id string) error
}
