package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nosleepman1/terangahost/internal/domain"
)

func atoi(s string) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	return v, err == nil
}

// DetectHardware interroge le serveur pour extraire ses caractéristiques matérielles réelles.
func DetectHardware(ctx context.Context, runner domain.Runner) (domain.HardwareSpec, error) {
	var spec domain.HardwareSpec
	read := func(cmd string) string {
		out, err := runner.RunSilent(ctx, cmd)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(out)
	}

	if v, ok := atoi(read("free -m | awk '/^Mem:/{print $2}'")); ok {
		spec.TotalRAMMB = v
	}
	if v, ok := atoi(read("free -m | awk '/^Mem:/{print $7}'")); ok {
		spec.FreeRAMMB = v // colonne "available"
	}
	if v, ok := atoi(read("free -m | awk '/^Swap:/{print $2}'")); ok {
		spec.TotalSwapMB = v
		spec.HasSwap = v >= 1000
	}
	if v, ok := atoi(read("nproc")); ok {
		spec.CPUCores = v
	}
	if spec.CPUCores == 0 {
		spec.CPUCores = 1
	}
	if f := strings.Fields(read("df -BG --output=size,avail / | tail -n 1 | tr -d G")); len(f) == 2 {
		spec.DiskTotalGB, _ = atoi(f[0])
		spec.DiskFreeGB, _ = atoi(f[1])
	}
	spec.OSVersion = read(`. /etc/os-release && echo "$PRETTY_NAME"`)
	spec.Architecture = read("uname -m")

	if spec.TotalRAMMB == 0 {
		return spec, fmt.Errorf("impossible de détecter la mémoire RAM du serveur")
	}
	return spec, nil
}

// DetectIPv6 retourne les adresses IPv6 globales du serveur (pour la vérification DNS AAAA).
func DetectIPv6(ctx context.Context, runner domain.Runner) []string {
	out, err := runner.RunSilent(ctx, "ip -6 -o addr show scope global 2>/dev/null | awk '{print $4}' | cut -d/ -f1")
	if err != nil {
		return nil
	}
	var ips []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			ips = append(ips, l)
		}
	}
	return ips
}
