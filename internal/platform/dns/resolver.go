package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/teranga-host/terangahost/internal/domain"
)

// PreFlightDNSCheck vérifie auprès des serveurs DNS mondiaux (Cloudflare 1.1.1.1 & Google 8.8.8.8)
// que le domaine configuré pointe bien vers l'adresse IP du serveur VPS cible.
// Cela protège l'utilisateur contre le bannissement Let's Encrypt (Rate Limit 5 échecs).
func PreFlightDNSCheck(ctx context.Context, domainName, expectedIP string) error {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			// Utiliser Cloudflare DNS 1.1.1.1:53 pour une résolution mondiale indépendante du cache local
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}

	cleanDomain := strings.TrimSpace(domainName)
	ips, err := resolver.LookupHost(ctx, cleanDomain)
	if err != nil {
		return fmt.Errorf("%w: impossible de résoudre le domaine %s (%v)", domain.ErrDNSPropagationPending, cleanDomain, err)
	}

	for _, ip := range ips {
		if strings.TrimSpace(ip) == strings.TrimSpace(expectedIP) {
			// Correspondance exacte trouvée
			return nil
		}
	}

	return fmt.Errorf("%w: le domaine %s pointe actuellement vers [%s], mais votre VPS est sur [%s]",
		domain.ErrDNSPropagationPending, cleanDomain, strings.Join(ips, ", "), expectedIP)
}
