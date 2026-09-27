package tests

import (
	"testing"

	"github.com/nosleepman1/terangahost/internal/domain"
)

func TestHardwareCalculations(t *testing.T) {
	cases := []struct {
		ram         int
		low         bool
		bufferPool  int
		fpmChildren int
		perSiteOf3  int
	}{
		{ram: 1024, low: true, bufferPool: 128, fpmChildren: 8, perSiteOf3: 3},
		{ram: 2048, low: false, bufferPool: 256, fpmChildren: 22, perSiteOf3: 7},
		{ram: 4096, low: false, bufferPool: 1024, fpmChildren: 42, perSiteOf3: 14},
		{ram: 16384, low: false, bufferPool: 6553, fpmChildren: 147, perSiteOf3: 49},
		{ram: 0, low: true, bufferPool: 128, fpmChildren: 8, perSiteOf3: 3}, // RAM inconnue : valeurs prudentes
	}
	for _, c := range cases {
		h := domain.HardwareSpec{TotalRAMMB: c.ram}
		if got := h.IsLowMemory(); got != c.low {
			t.Errorf("%d Mo: IsLowMemory = %v", c.ram, got)
		}
		if got := h.TunedMySQLBufferPoolMB(); got != c.bufferPool {
			t.Errorf("%d Mo: buffer pool = %d, attendu %d", c.ram, got, c.bufferPool)
		}
		if got := h.TunedFpmMaxChildren(); got != c.fpmChildren {
			t.Errorf("%d Mo: FPM max children = %d, attendu %d", c.ram, got, c.fpmChildren)
		}
		if got := h.FpmMaxChildrenPerSite(3); got != c.perSiteOf3 {
			t.Errorf("%d Mo: FPM par site (3 sites) = %d, attendu %d", c.ram, got, c.perSiteOf3)
		}
	}
}
