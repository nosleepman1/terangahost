package domain

// HardwareSpec représente les caractéristiques physiques réelles détectées sur le VPS
type HardwareSpec struct {
	TotalRAMMB   int    `json:"total_ram_mb"`   // Ex: 1024 (1 Go) ou 4096 (4 Go)
	FreeRAMMB    int    `json:"free_ram_mb"`
	CPUCores     int    `json:"cpu_cores"`      // Ex: 1, 2, 4
	TotalSwapMB  int    `json:"total_swap_mb"`  // Ex: 2048
	HasSwap      bool   `json:"has_swap"`       // true si swap > 500Mo
	DiskTotalGB  int    `json:"disk_total_gb"`  // Ex: 25
	DiskFreeGB   int    `json:"disk_free_gb"`   // Ex: 18
	OSVersion    string `json:"os_version"`     // Ex: "Ubuntu 24.04 LTS"
	Architecture string `json:"architecture"`   // Ex: "x86_64" ou "aarch64"
}

// IsLowMemory renvoie true si le serveur a 1 Go ou moins de RAM
func (h HardwareSpec) IsLowMemory() bool {
	return h.TotalRAMMB <= 1200
}

// TunedMySQLBufferPoolMB calcule dynamiquement la taille idéale du buffer MySQL
func (h HardwareSpec) TunedMySQLBufferPoolMB() int {
	if h.TotalRAMMB <= 1200 {
		return 128 // Protection maximale contre l'OOM sur 1GB VPS
	}
	if h.TotalRAMMB <= 2400 {
		return 256
	}
	if h.TotalRAMMB <= 4800 {
		return 1024 // 1 Go sur un VPS 4 Go
	}
	return (h.TotalRAMMB * 40) / 100 // 40% de la RAM sur les gros serveurs
}

// TunedFpmMaxChildren calcule le nombre de processus FPM recommandés
func (h HardwareSpec) TunedFpmMaxChildren() int {
	if h.TotalRAMMB <= 1200 {
		return 5
	}
	if h.TotalRAMMB <= 2400 {
		return 12
	}
	if h.TotalRAMMB <= 4800 {
		return 25
	}
	return (h.TotalRAMMB / 100)
}
