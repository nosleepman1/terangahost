package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/teranga-host/terangahost/internal/domain"
)

type LocalData struct {
	Servers []*domain.Server `json:"servers"`
	Sites   []*domain.Site   `json:"sites"`
}

// JSONRepository implémente ServerRepository et SiteRepository avec persistance dans ~/.terangahost/config.json
type JSONRepository struct {
	filePath string
	mu       sync.RWMutex
}

// NewJSONRepository initialise le répertoire local ~/.terangahost/
func NewJSONRepository() (*JSONRepository, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("impossible de localiser le répertoire utilisateur: %w", err)
	}

	dir := filepath.Join(home, ".terangahost")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("impossible de créer le répertoire ~/.terangahost: %w", err)
	}

	filePath := filepath.Join(dir, "config.json")
	repo := &JSONRepository{filePath: filePath}

	// Créer le fichier s'il n'existe pas encore
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		initialData := LocalData{
			Servers: make([]*domain.Server, 0),
			Sites:   make([]*domain.Site, 0),
		}
		raw, _ := json.MarshalIndent(initialData, "", "  ")
		if err := os.WriteFile(filePath, raw, 0600); err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func (r *JSONRepository) load() (*LocalData, error) {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return nil, err
	}
	var out LocalData
	if err := json.Unmarshal(data, &out); err != nil {
		return &LocalData{Servers: []*domain.Server{}, Sites: []*domain.Site{}}, nil
	}
	return &out, nil
}

func (r *JSONRepository) save(data *LocalData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, raw, 0600)
}

// Save enregistre ou met à jour un serveur
func (r *JSONRepository) Save(ctx context.Context, server *domain.Server) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := r.load()
	if err != nil {
		return err
	}

	found := false
	for i, s := range data.Servers {
		if s.ID == server.ID || s.Name == server.Name {
			data.Servers[i] = server
			found = true
			break
		}
	}

	if !found {
		data.Servers = append(data.Servers, server)
	}

	return r.save(data)
}

// FindByID cherche un serveur par son identifiant unique
func (r *JSONRepository) FindByID(ctx context.Context, id string) (*domain.Server, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := r.load()
	if err != nil {
		return nil, err
	}

	for _, s := range data.Servers {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, domain.ErrServerNotFound
}

// FindByName cherche un serveur par son nom
func (r *JSONRepository) FindByName(ctx context.Context, name string) (*domain.Server, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := r.load()
	if err != nil {
		return nil, err
	}

	for _, s := range data.Servers {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, domain.ErrServerNotFound
}

// FindByIP cherche un serveur par son adresse IP
func (r *JSONRepository) FindByIP(ctx context.Context, ip string) (*domain.Server, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := r.load()
	if err != nil {
		return nil, err
	}

	for _, s := range data.Servers {
		if s.IP == ip {
			return s, nil
		}
	}
	return nil, domain.ErrServerNotFound
}

// List retourne tous les serveurs enregistrés
func (r *JSONRepository) List(ctx context.Context) ([]*domain.Server, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := r.load()
	if err != nil {
		return nil, err
	}
	return data.Servers, nil
}

// Delete supprime un serveur
func (r *JSONRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	data, err := r.load()
	if err != nil {
		return err
	}

	newServers := make([]*domain.Server, 0)
	for _, s := range data.Servers {
		if s.ID != id && s.Name != id {
			newServers = append(newServers, s)
		}
	}
	data.Servers = newServers
	return r.save(data)
}
