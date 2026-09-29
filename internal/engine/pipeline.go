// Package engine orchestre l'exécution séquentielle et idempotente des étapes de provisionnement.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/ui"
)

// PipelineListener reçoit les événements d'exécution (affichage terminal, tests...).
type PipelineListener interface {
	OnStepStart(index, total int, step domain.Step)
	OnStepSuccess(index, total int, step domain.Step, duration time.Duration, skipped bool)
	OnStepFailure(index, total int, step domain.Step, err error)
}

// Pipeline exécute les étapes dans l'ordre et s'arrête à la première erreur.
type Pipeline struct {
	steps    []domain.Step
	listener PipelineListener
	log      *slog.Logger
}

// NewPipeline initialise le pipeline. listener et log peuvent être nil.
func NewPipeline(listener PipelineListener, log *slog.Logger) *Pipeline {
	return &Pipeline{listener: listener, log: log}
}

// AddStep ajoute une étape au pipeline.
func (p *Pipeline) AddStep(steps ...domain.Step) {
	p.steps = append(p.steps, steps...)
}

// Steps retourne les étapes enregistrées.
func (p *Pipeline) Steps() []domain.Step { return p.steps }

// Execute lance toutes les étapes dans l'ordre.
func (p *Pipeline) Execute(ctx context.Context, runner domain.Runner, server *domain.Server) error {
	total := len(p.steps)
	for i, step := range p.steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		index := i + 1
		if p.listener != nil {
			p.listener.OnStepStart(index, total, step)
		}
		start := time.Now()

		satisfied, err := step.PreCheck(ctx, runner, server)
		if err != nil {
			// Un PreCheck en erreur n'est pas bloquant : l'étape est simplement (ré)exécutée.
			p.logWarn("PreCheck en erreur, exécution de l'étape", step, err)
			satisfied = false
		}
		if satisfied {
			if p.listener != nil {
				p.listener.OnStepSuccess(index, total, step, time.Since(start), true)
			}
			continue
		}

		if p.log != nil {
			p.log.Info("début de l'étape", "step", step.ID(), "title", step.Title())
		}
		if err := step.Execute(ctx, runner, server); err != nil {
			if p.listener != nil {
				p.listener.OnStepFailure(index, total, step, err)
			}
			if p.log != nil {
				p.log.Error("échec de l'étape", "step", step.ID(), "error", err)
			}
			return fmt.Errorf("étape [%s] %s: %w", step.ID(), step.Title(), err)
		}
		if p.listener != nil {
			p.listener.OnStepSuccess(index, total, step, time.Since(start), false)
		}
	}
	return nil
}

func (p *Pipeline) logWarn(msg string, step domain.Step, err error) {
	if p.log != nil {
		p.log.Warn(msg, "step", step.ID(), "error", err)
	}
}

// ConsoleListener affiche la progression avec un spinner.
type ConsoleListener struct {
	spinner *ui.Spinner
}

func label(index, total int, step domain.Step) string {
	return fmt.Sprintf("%s %s", ui.Gray(fmt.Sprintf("[%d/%d]", index, total)), step.Title())
}

func (c *ConsoleListener) OnStepStart(index, total int, step domain.Step) {
	c.spinner = ui.StartSpinner(label(index, total, step))
}

func (c *ConsoleListener) OnStepSuccess(index, total int, step domain.Step, _ time.Duration, skipped bool) {
	if skipped {
		c.spinner.Skip(label(index, total, step))
		return
	}
	c.spinner.Succeed(label(index, total, step))
}

func (c *ConsoleListener) OnStepFailure(index, total int, step domain.Step, _ error) {
	c.spinner.Fail(label(index, total, step))
}
