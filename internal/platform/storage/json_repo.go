// Package storage persiste l'inventaire local (serveurs et sites) dans ~/.terangahost/config.json.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// LocalData est le contenu du fichier de configuration.
type LocalData struct {
	Servers []*domain.Server `json:"servers"`
	Sites   []*domain.Site   `json:"sites"`
}

// JSONRepository implémente domain.Store.
// Les écritures sont atomiques (fichier temporaire + rename) et protégées par un fichier verrou
// afin que deux commandes lancées en parallèle ne s'écrasent pas.
type JSONRepository struct {
	filePath string
}

var _ domain.Store = (*JSONRepository)(nil)

// DefaultDir retourne ~/.terangahost (ou $TERANGAHOST_HOME).
func DefaultDir() (string, error) {
	if dir := os.Getenv("TERANGAHOST_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("impossible de localiser le répertoire utilisateur: %w", err)
	}
	return filepath.Join(home, ".terangahost"), nil
}

// NewJSONRepository ouvre le dépôt dans le dossier par défaut.
func NewJSONRepository() (*JSONRepository, error) {
	dir, err := DefaultDir()
	if err != nil {
		return nil, err
	}
	return NewJSONRepositoryAt(dir)
}

// NewJSONRepositoryAt ouvre le dépôt dans un dossier donné.
func NewJSONRepositoryAt(dir string) (*JSONRepository, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("impossible de créer le répertoire %s: %w", dir, err)
	}
	return &JSONRepository{filePath: filepath.Join(dir, "config.json")}, nil
}

// Path retourne le chemin du fichier de configuration.
func (r *JSONRepository) Path() string { return r.filePath }

func (r *JSONRepository) load() (*LocalData, error) {
	data, err := os.ReadFile(r.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return &LocalData{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out LocalData
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("%w (%s): %v. Corrigez le fichier ou restaurez %s.bak", domain.ErrConfigCorrupted, r.filePath, err, r.filePath)
	}
	return &out, nil
}

func (r *JSONRepository) write(data *LocalData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if prev, err := os.ReadFile(r.filePath); err == nil && json.Valid(prev) {
		_ = os.WriteFile(r.filePath+".bak", prev, 0o600)
	}
	tmp, err := os.CreateTemp(filepath.Dir(r.filePath), ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), r.filePath)
}

// lock acquiert un verrou inter-processus basé sur la création exclusive d'un fichier.
func (r *JSONRepository) lock(ctx context.Context) (func(), error) {
	lockPath := r.filePath + ".lock"
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		// Verrou orphelin laissé par un processus interrompu.
		if st, statErr := os.Stat(lockPath); statErr == nil && time.Since(st.ModTime()) > 30*time.Second {
			os.Remove(lockPath)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("configuration verrouillée par une autre commande TerangaHost (%s)", lockPath)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (r *JSONRepository) mutate(ctx context.Context, fn func(*LocalData) error) error {
	unlock, err := r.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	data, err := r.load()
	if err != nil {
		return err
	}
	if err := fn(data); err != nil {
		return err
	}
	return r.write(data)
}

// SaveServer enregistre ou met à jour un serveur (clé : ID, sinon nom).
func (r *JSONRepository) SaveServer(ctx context.Context, server *domain.Server) error {
	server.UpdatedAt = time.Now()
	return r.mutate(ctx, func(d *LocalData) error {
		for i, s := range d.Servers {
			if s.ID == server.ID || s.Name == server.Name {
				d.Servers[i] = server
				return nil
			}
		}
		d.Servers = append(d.Servers, server)
		return nil
	})
}

// FindServer cherche un serveur par son nom.
func (r *JSONRepository) FindServer(_ context.Context, name string) (*domain.Server, error) {
	d, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, s := range d.Servers {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", domain.ErrServerNotFound, name)
}

// FindServerByID cherche un serveur par son identifiant.
func (r *JSONRepository) FindServerByID(_ context.Context, id string) (*domain.Server, error) {
	d, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, s := range d.Servers {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: id %q", domain.ErrServerNotFound, id)
}

// ListServers retourne les serveurs triés par nom.
func (r *JSONRepository) ListServers(_ context.Context) ([]*domain.Server, error) {
	d, err := r.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(d.Servers, func(i, j int) bool { return d.Servers[i].Name < d.Servers[j].Name })
	return d.Servers, nil
}

// DeleteServer supprime un serveur et ses sites.
func (r *JSONRepository) DeleteServer(ctx context.Context, name string) error {
	return r.mutate(ctx, func(d *LocalData) error {
		var id string
		servers := d.Servers[:0]
		for _, s := range d.Servers {
			if s.Name == name {
				id = s.ID
				continue
			}
			servers = append(servers, s)
		}
		if id == "" {
			return fmt.Errorf("%w: %q", domain.ErrServerNotFound, name)
		}
		d.Servers = servers
		sites := d.Sites[:0]
		for _, s := range d.Sites {
			if s.ServerID != id {
				sites = append(sites, s)
			}
		}
		d.Sites = sites
		return nil
	})
}

// SaveSite enregistre ou met à jour un site (clé : domaine).
func (r *JSONRepository) SaveSite(ctx context.Context, site *domain.Site) error {
	site.UpdatedAt = time.Now()
	return r.mutate(ctx, func(d *LocalData) error {
		for i, s := range d.Sites {
			if s.Domain == site.Domain {
				d.Sites[i] = site
				return nil
			}
		}
		d.Sites = append(d.Sites, site)
		return nil
	})
}

// FindSite cherche un site par domaine.
func (r *JSONRepository) FindSite(_ context.Context, domainName string) (*domain.Site, error) {
	d, err := r.load()
	if err != nil {
		return nil, err
	}
	for _, s := range d.Sites {
		if s.Domain == domainName {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", domain.ErrSiteNotFound, domainName)
}

// ListSites retourne les sites d'un serveur (ou tous), triés par domaine.
func (r *JSONRepository) ListSites(_ context.Context, serverID string) ([]*domain.Site, error) {
	d, err := r.load()
	if err != nil {
		return nil, err
	}
	var out []*domain.Site
	for _, s := range d.Sites {
		if serverID == "" || s.ServerID == serverID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Domain < out[j].Domain })
	return out, nil
}

// DeleteSite supprime un site.
func (r *JSONRepository) DeleteSite(ctx context.Context, domainName string) error {
	return r.mutate(ctx, func(d *LocalData) error {
		sites := d.Sites[:0]
		found := false
		for _, s := range d.Sites {
			if s.Domain == domainName {
				found = true
				continue
			}
			sites = append(sites, s)
		}
		if !found {
			return fmt.Errorf("%w: %q", domain.ErrSiteNotFound, domainName)
		}
		d.Sites = sites
		return nil
	})
}
