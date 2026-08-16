package tests

import (
	"context"
	"testing"
	"time"

	"github.com/teranga-host/terangahost/internal/domain"
	"github.com/teranga-host/terangahost/internal/engine"
	"github.com/teranga-host/terangahost/internal/engine/steps"
	"github.com/teranga-host/terangahost/tests/mocks"
)

func TestPipelineExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mockRunner := mocks.NewMockRunner()
	mockRunner.CmdOutputs["cat /etc/os-release"] = "NAME=\"Ubuntu\"\nVERSION=\"24.04 LTS (Noble Numbat)\""
	mockRunner.CmdOutputs["fuser"] = "FREE"

	server := &domain.Server{
		ID:         "srv_test_01",
		Name:       "test-vps",
		IP:         "127.0.0.1",
		SSHPort:    22,
		RootUser:   "root",
		DeployUser: "deployer",
		PHPVersion: "8.3",
		Database:   "mariadb",
		WithRedis:  true,
		Hardware: domain.HardwareSpec{
			TotalRAMMB: 2048,
			CPUCores:   2,
		},
	}

	pipeline := engine.NewPipeline(nil)
	pipeline.AddStep(&steps.StepHandshake{})
	pipeline.AddStep(&steps.StepSwap{})
	pipeline.AddStep(&steps.StepSecurity{})
	pipeline.AddStep(&steps.StepSudoers{})
	pipeline.AddStep(&steps.StepPHP{})
	pipeline.AddStep(&steps.StepWebServer{})
	pipeline.AddStep(&steps.StepTools{})
	pipeline.AddStep(&steps.StepDatabase{})

	err := pipeline.Execute(ctx, mockRunner, server, nil)
	if err != nil {
		t.Fatalf("Pipeline execution failed: %v", err)
	}

	// Vérifications sur les commandes exécutées
	if !mockRunner.HasExecuted("swapon /swapfile") {
		t.Errorf("Expected swap setup command to be executed")
	}

	if !mockRunner.HasExecuted("php8.3") {
		t.Errorf("Expected PHP 8.3 installation command to be executed")
	}

	if !mockRunner.HasExecuted("nginx") {
		t.Errorf("Expected Nginx command to be executed")
	}

	if !mockRunner.HasExecuted("mariadb-server") {
		t.Errorf("Expected MariaDB command to be executed")
	}

	// Vérifier l'upload du sudoers
	exists, err := mockRunner.FileExists(ctx, "/etc/sudoers.d/terangahost-deployer")
	if err != nil || !exists {
		t.Errorf("Expected sudoers file to be uploaded")
	}
}
