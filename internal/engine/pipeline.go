package engine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/fatih/color"
	"github.com/teranga-host/terangahost/internal/domain"
	"github.com/teranga-host/terangahost/internal/platform/logger"
)

// PipelineListener permet à l'interface terminale (TUI) de recevoir les événements en temps réel
type PipelineListener interface {
	OnStepStart(index, total int, step domain.Step)
	OnStepSuccess(index, total int, step domain.Step, duration time.Duration, skipped bool)
	OnStepFailure(index, total int, step domain.Step, err error)
}

// Pipeline orchestre l'exécution séquentielle et sécurisée des étapes de configuration
type Pipeline struct {
	steps    []domain.Step
	listener PipelineListener
}

// NewPipeline initialise le pipeline
func NewPipeline(listener PipelineListener) *Pipeline {
	return &Pipeline{
		steps:    make([]domain.Step, 0),
		listener: listener,
	}
}

// AddStep ajoute une étape au pipeline
func (p *Pipeline) AddStep(step domain.Step) {
	p.steps = append(p.steps, step)
}

// Execute lance toutes les étapes dans l'ordre
func (p *Pipeline) Execute(ctx context.Context, runner domain.Runner, server *domain.Server, logFile *logger.FileLogger) error {
	total := len(p.steps)

	for i, step := range p.steps {
		index := i + 1

		if p.listener != nil {
			p.listener.OnStepStart(index, total, step)
		}

		startTime := time.Now()

		// 1. PreCheck d'Idempotence : Vérifier si l'étape est déjà exécutée
		isSatisfied, err := step.PreCheck(ctx, runner, server)
		if err != nil {
			if logFile != nil {
				logFile.Logger.Warn("PreCheck warning", "step", step.ID(), "error", err)
			}
		}

		if isSatisfied {
			duration := time.Since(startTime)
			if p.listener != nil {
				p.listener.OnStepSuccess(index, total, step, duration, true)
			}
			continue
		}

		// 2. Exécution de l'étape
		if logFile != nil {
			logFile.Logger.Info("Démarrage de l'étape", "step", step.ID(), "title", step.Title())
		}

		stepErr := step.Execute(ctx, runner, server)
		duration := time.Since(startTime)

		if stepErr != nil {
			if p.listener != nil {
				p.listener.OnStepFailure(index, total, step, stepErr)
			}
			if logFile != nil {
				logFile.Logger.Error("Échec de l'étape", "step", step.ID(), "error", stepErr)
			}
			return fmt.Errorf("échec de l'étape [%s - %s]: %w", step.ID(), step.Title(), stepErr)
		}

		if p.listener != nil {
			p.listener.OnStepSuccess(index, total, step, duration, false)
		}
	}

	return nil
}

// DefaultConsoleListener est une implémentation par défaut pour afficher les étapes
type DefaultConsoleListener struct {
	Writer io.Writer
}

func (d *DefaultConsoleListener) OnStepStart(index, total int, step domain.Step) {
	cyan := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Fprintf(d.Writer, "  %s %s...\n", cyan(fmt.Sprintf("[%d/%d]", index, total)), step.Title())
}

func (d *DefaultConsoleListener) OnStepSuccess(index, total int, step domain.Step, duration time.Duration, skipped bool) {
	if skipped {
		yellow := color.New(color.FgYellow).SprintFunc()
		fmt.Fprintf(d.Writer, "    %s Déjà configuré (sauté en %s)\n", yellow("⚡ [SKIPPED]"), duration.Round(time.Millisecond))
	} else {
		green := color.New(color.FgGreen, color.Bold).SprintFunc()
		fmt.Fprintf(d.Writer, "    %s Terminé avec succès en %s\n\n", green("✔ [OK]"), duration.Round(time.Millisecond))
	}
}

func (d *DefaultConsoleListener) OnStepFailure(index, total int, step domain.Step, err error) {
	red := color.New(color.FgRed, color.Bold).SprintFunc()
	fmt.Fprintf(d.Writer, "    %s Erreur: %v\n\n", red("✖ [FAILED]"), err)
}
