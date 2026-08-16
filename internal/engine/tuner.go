package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/teranga-host/terangahost/internal/domain"
)

// DetectHardware interroge le système distant pour extraire les caractéristiques matérielles réelles
func DetectHardware(ctx context.Context, runner domain.Runner) (domain.HardwareSpec, error) {
	var spec domain.HardwareSpec

	// 1. RAM totale en Mo
	ramCmd := "free -m | awk '/^Mem:/{print $2}'"
	ramOut, err := runner.RunSilent(ctx, ramCmd)
	if err == nil {
		if ram, err := strconv.Atoi(strings.TrimSpace(ramOut)); err == nil {
			spec.TotalRAMMB = ram
		}
	}

	// 2. RAM libre en Mo
	freeRamCmd := "free -m | awk '/^Mem:/{print $4+$6+$7}'" // Free + buff/cache
	freeRamOut, err := runner.RunSilent(ctx, freeRamCmd)
	if err == nil {
		if free, err := strconv.Atoi(strings.TrimSpace(freeRamOut)); err == nil {
			spec.FreeRAMMB = free
		}
	}

	// 3. Swap existant en Mo
	swapCmd := "free -m | awk '/^Swap:/{print $2}'"
	swapOut, err := runner.RunSilent(ctx, swapCmd)
	if err == nil {
		if swap, err := strconv.Atoi(strings.TrimSpace(swapOut)); err == nil {
			spec.TotalSwapMB = swap
			spec.HasSwap = swap > 500
		}
	}

	// 4. Nombre de cœurs CPU
	cpuCmd := "nproc"
	cpuOut, err := runner.RunSilent(ctx, cpuCmd)
	if err == nil {
		if cpu, err := strconv.Atoi(strings.TrimSpace(cpuOut)); err == nil {
			spec.CPUCores = cpu
		}
	}
	if spec.CPUCores == 0 {
		spec.CPUCores = 1
	}

	// 5. Espace disque restant en Go
	diskCmd := "df -BG / | awk 'NR==2 {gsub(\"G\", \"\", $4); print $4}'"
	diskOut, err := runner.RunSilent(ctx, diskCmd)
	if err == nil {
		if disk, err := strconv.Atoi(strings.TrimSpace(diskOut)); err == nil {
			spec.DiskFreeGB = disk
		}
	}

	// 6. Version d'OS
	osCmd := "lsb_release -ds 2>/dev/null || cat /etc/os-release | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"'"
	osOut, err := runner.RunSilent(ctx, osCmd)
	if err == nil {
		spec.OSVersion = strings.TrimSpace(osOut)
	}

	// 7. Architecture
	archCmd := "uname -m"
	archOut, err := runner.RunSilent(ctx, archCmd)
	if err == nil {
		spec.Architecture = strings.TrimSpace(archOut)
	}

	if spec.TotalRAMMB == 0 {
		return spec, fmt.Errorf("impossible de détecter la mémoire RAM du serveur")
	}

	return spec, nil
}
