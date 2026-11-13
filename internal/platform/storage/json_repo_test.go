package storage

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/nosleepman1/terangahost/internal/domain"
)

func TestServerAndSiteLifecycle(t *testing.T) {
	ctx := context.Background()
	repo, err := NewJSONRepositoryAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindServer(ctx, "x"); !errors.Is(err, domain.ErrServerNotFound) {
		t.Fatalf("attendu ErrServerNotFound, obtenu %v", err)
	}

	srv := &domain.Server{ID: "srv_1", Name: "prod", IP: "10.0.0.1"}
	if err := repo.SaveServer(ctx, srv); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSite(ctx, &domain.Site{ID: "a", ServerID: "srv_1", Domain: "a.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveSite(ctx, &domain.Site{ID: "b", ServerID: "other", Domain: "b.example.com"}); err != nil {
		t.Fatal(err)
	}

	sites, _ := repo.ListSites(ctx, "srv_1")
	if len(sites) != 1 || sites[0].Domain != "a.example.com" {
		t.Fatalf("ListSites filtré incorrect: %+v", sites)
	}

	// Mise à jour par nom : pas de doublon.
	srv.IP = "10.0.0.2"
	_ = repo.SaveServer(ctx, srv)
	servers, _ := repo.ListServers(ctx)
	if len(servers) != 1 || servers[0].IP != "10.0.0.2" {
		t.Fatalf("mise à jour incorrecte: %+v", servers)
	}

	if err := repo.DeleteServer(ctx, "prod"); err != nil {
		t.Fatal(err)
	}
	all, _ := repo.ListSites(ctx, "")
	if len(all) != 1 || all[0].Domain != "b.example.com" {
		t.Fatalf("la suppression du serveur doit supprimer ses sites: %+v", all)
	}
}

func TestCorruptedConfigIsNeverOverwritten(t *testing.T) {
	ctx := context.Background()
	repo, _ := NewJSONRepositoryAt(t.TempDir())
	if err := os.WriteFile(repo.Path(), []byte("{corrompu"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ListServers(ctx); !errors.Is(err, domain.ErrConfigCorrupted) {
		t.Fatalf("attendu ErrConfigCorrupted, obtenu %v", err)
	}
	if err := repo.SaveServer(ctx, &domain.Server{ID: "x", Name: "x"}); err == nil {
		t.Fatal("l'écriture doit échouer sur une configuration corrompue")
	}
	raw, _ := os.ReadFile(repo.Path())
	if string(raw) != "{corrompu" {
		t.Fatal("le fichier corrompu a été écrasé")
	}
}

func TestConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			repo, _ := NewJSONRepositoryAt(dir)
			name := string(rune('a'+i)) + "-srv"
			if err := repo.SaveServer(ctx, &domain.Server{ID: name, Name: name}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	repo, _ := NewJSONRepositoryAt(dir)
	servers, err := repo.ListServers(ctx)
	if err != nil || len(servers) != 20 {
		t.Fatalf("attendu 20 serveurs, obtenu %d (%v)", len(servers), err)
	}
}
