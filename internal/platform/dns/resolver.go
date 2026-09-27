// Package dns vérifie la propagation DNS avant toute demande de certificat Let's Encrypt.
package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
)

// Resolvers publics interrogés directement, indépendamment du cache local.
var Resolvers = []string{"1.1.1.1:53", "8.8.8.8:53"}

// LookupFunc résout un nom en adresses IP via un serveur DNS donné (remplaçable dans les tests).
var LookupFunc = func(ctx context.Context, server, host string) ([]string, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, server)
		},
	}
	return r.LookupHost(ctx, host)
}

// PreFlightDNSCheck vérifie, auprès de chaque resolver public, que toutes les adresses IPv4 du domaine
// pointent vers expectedIP et que les éventuelles adresses IPv6 correspondent au serveur.
// Let's Encrypt privilégie l'IPv6 : un enregistrement AAAA erroné fait échouer la validation.
func PreFlightDNSCheck(ctx context.Context, domainName, expectedIP string, serverIPv6 []string) error {
	var lastErr error
	for _, resolver := range Resolvers {
		ips, err := LookupFunc(ctx, resolver, domainName)
		if err != nil {
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
				return fmt.Errorf("%w: aucun enregistrement DNS pour %s", domain.ErrDNSPropagationPending, domainName)
			}
			lastErr = err
			continue
		}
		if err := compare(domainName, ips, expectedIP, serverIPv6); err != nil {
			return fmt.Errorf("%w (resolver %s)", err, resolver)
		}
	}
	if lastErr != nil {
		return fmt.Errorf("%w: résolution de %s impossible: %v", domain.ErrDNSPropagationPending, domainName, lastErr)
	}
	return nil
}

func compare(domainName string, ips []string, expectedIP string, serverIPv6 []string) error {
	expected := net.ParseIP(expectedIP)
	var v4, v6 []string
	for _, raw := range ips {
		ip := net.ParseIP(strings.TrimSpace(raw))
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			v4 = append(v4, ip.String())
		} else {
			v6 = append(v6, ip.String())
		}
	}

	if expected != nil && expected.To4() != nil {
		if len(v4) == 0 {
			return fmt.Errorf("%w: aucun enregistrement A pour %s (attendu %s)", domain.ErrDNSPropagationPending, domainName, expectedIP)
		}
		for _, ip := range v4 {
			if ip != expected.String() {
				return fmt.Errorf("%w: %s pointe vers [%s] alors que le VPS est %s", domain.ErrDNSPropagationPending, domainName, strings.Join(v4, ", "), expectedIP)
			}
		}
	}

	allowed := map[string]bool{}
	for _, ip := range serverIPv6 {
		if p := net.ParseIP(ip); p != nil {
			allowed[p.String()] = true
		}
	}
	if expected != nil && expected.To4() == nil {
		allowed[expected.String()] = true
	}
	for _, ip := range v6 {
		if !allowed[ip] {
			return fmt.Errorf("%w: l'enregistrement AAAA de %s (%s) ne correspond pas au VPS ; supprimez-le ou corrigez-le",
				domain.ErrDNSPropagationPending, domainName, ip)
		}
	}
	if expected != nil && expected.To4() == nil && !allowed[expected.String()] {
		return fmt.Errorf("%w: aucun enregistrement AAAA correct pour %s", domain.ErrDNSPropagationPending, domainName)
	}
	return nil
}
