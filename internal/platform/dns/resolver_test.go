package dns

import (
	"context"
	"errors"
	"testing"

	"github.com/nosleepman1/terangahost/internal/domain"
)

func withLookup(t *testing.T, ips []string) {
	old := LookupFunc
	LookupFunc = func(context.Context, string, string) ([]string, error) { return ips, nil }
	t.Cleanup(func() { LookupFunc = old })
}

func TestPreFlightDNSCheck(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		ips    []string
		v6     []string
		wantOK bool
	}{
		{"A correct", []string{"203.0.113.10"}, nil, true},
		{"A incorrect", []string{"198.51.100.1"}, nil, false},
		{"A multiples dont un faux", []string{"203.0.113.10", "198.51.100.1"}, nil, false},
		{"AAAA inconnu", []string{"203.0.113.10", "2001:db8::99"}, nil, false},
		{"AAAA du serveur", []string{"203.0.113.10", "2001:db8::1"}, []string{"2001:db8::1"}, true},
		{"aucun A", []string{"2001:db8::1"}, []string{"2001:db8::1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withLookup(t, c.ips)
			err := PreFlightDNSCheck(ctx, "api.example.com", "203.0.113.10", c.v6)
			if c.wantOK && err != nil {
				t.Fatalf("erreur inattendue: %v", err)
			}
			if !c.wantOK && !errors.Is(err, domain.ErrDNSPropagationPending) {
				t.Fatalf("attendu ErrDNSPropagationPending, obtenu %v", err)
			}
		})
	}
}
