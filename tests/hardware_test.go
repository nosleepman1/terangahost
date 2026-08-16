package tests

import (
	"testing"

	"github.com/teranga-host/terangahost/internal/domain"
)

func TestHardwareCalculations(t *testing.T) {
	t.Run("Low RAM VPS (1GB) calculation", func(t *testing.T) {
		spec := domain.HardwareSpec{
			TotalRAMMB: 1024,
			CPUCores:   1,
		}

		if !spec.IsLowMemory() {
			t.Errorf("Expected IsLowMemory to be true for 1024MB RAM")
		}

		if pool := spec.TunedMySQLBufferPoolMB(); pool != 128 {
			t.Errorf("Expected MySQL Buffer Pool to be 128MB, got %dMB", pool)
		}

		if fpm := spec.TunedFpmMaxChildren(); fpm != 5 {
			t.Errorf("Expected FPM Max Children to be 5, got %d", fpm)
		}
	})

	t.Run("Medium RAM VPS (4GB) calculation", func(t *testing.T) {
		spec := domain.HardwareSpec{
			TotalRAMMB: 4096,
			CPUCores:   2,
		}

		if spec.IsLowMemory() {
			t.Errorf("Expected IsLowMemory to be false for 4096MB RAM")
		}

		if pool := spec.TunedMySQLBufferPoolMB(); pool != 1024 {
			t.Errorf("Expected MySQL Buffer Pool to be 1024MB, got %dMB", pool)
		}

		if fpm := spec.TunedFpmMaxChildren(); fpm != 25 {
			t.Errorf("Expected FPM Max Children to be 25, got %d", fpm)
		}
	})
}
