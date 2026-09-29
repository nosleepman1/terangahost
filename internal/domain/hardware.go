package domain

// HardwareSpec représente les caractéristiques physiques réelles détectées sur le VPS.
type HardwareSpec struct {
	TotalRAMMB   int    `json:"total_ram_mb"`
	FreeRAMMB    int    `json:"free_ram_mb"`
	CPUCores     int    `json:"cpu_cores"`
	TotalSwapMB  int    `json:"total_swap_mb"`
	HasSwap      bool   `json:"has_swap"`
	DiskTotalGB  int    `json:"disk_total_gb"`
	DiskFreeGB   int    `json:"disk_free_gb"`
	OSVersion    string `json:"os_version"`
	Architecture string `json:"architecture"`
}

// Consommation mémoire moyenne d'un worker PHP-FPM Laravel et mémoire réservée au système.
const (
	fpmWorkerMB  = 64
	systemBaseMB = 384
)

func (h HardwareSpec) ramMB() int {
	if h.TotalRAMMB <= 0 {
		return 1024
	}
	return h.TotalRAMMB
}

// IsLowMemory renvoie true si le serveur a environ 1 Go de RAM ou moins.
func (h HardwareSpec) IsLowMemory() bool {
	return h.ramMB() <= 1200
}

// TunedMySQLBufferPoolMB calcule la taille du buffer pool InnoDB adaptée à la RAM réelle.
func (h HardwareSpec) TunedMySQLBufferPoolMB() int {
	ram := h.ramMB()
	switch {
	case ram <= 1200:
		return 128 // protection contre l'OOM killer sur un VPS 1 Go
	case ram <= 2400:
		return 256
	case ram <= 4800:
		return 1024
	default:
		return ram * 40 / 100
	}
}

// TunedFpmMaxChildren calcule le budget total de processus PHP-FPM du serveur :
// la RAM restante après le système et la base de données, divisée par la consommation d'un worker.
func (h HardwareSpec) TunedFpmMaxChildren() int {
	available := h.ramMB() - systemBaseMB - h.TunedMySQLBufferPoolMB()
	n := available / fpmWorkerMB
	if n < 4 {
		n = 4
	}
	if n > 256 {
		n = 256
	}
	return n
}

// FpmMaxChildrenPerSite répartit le budget FPM entre les sites hébergés pour éviter
// qu'un pic de charge simultané sur plusieurs sites ne dépasse la mémoire disponible.
func (h HardwareSpec) FpmMaxChildrenPerSite(sites int) int {
	if sites < 1 {
		sites = 1
	}
	n := h.TunedFpmMaxChildren() / sites
	if n < 3 {
		n = 3
	}
	return n
}
