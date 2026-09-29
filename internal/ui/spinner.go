package ui

import (
	"fmt"
	"sync"
	"time"
)

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerEnabled permet de désactiver l'animation (mode verbeux, sortie non interactive).
var SpinnerEnabled = true

// Spinner anime une ligne de progression pendant une opération longue.
// Hors terminal, il affiche simplement le message une fois.
type Spinner struct {
	mu      sync.Mutex
	msg     string
	start   time.Time
	stop    chan struct{}
	done    chan struct{}
	animate bool
}

// StartSpinner démarre un indicateur de progression.
func StartSpinner(msg string) *Spinner {
	s := &Spinner{msg: msg, start: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	s.animate = SpinnerEnabled && IsTTY()
	if !s.animate {
		fmt.Fprintf(Out, "  … %s\n", msg)
		close(s.done)
		return s
	}
	go s.loop()
	return s
}

func (s *Spinner) loop() {
	defer close(s.done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for i := 0; ; i++ {
		s.mu.Lock()
		fmt.Fprintf(Out, "\r\033[K  %s %s %s", Cyan(frames[i%len(frames)]), s.msg, Gray(Elapsed(s.start)))
		s.mu.Unlock()
		select {
		case <-s.stop:
			fmt.Fprint(Out, "\r\033[K")
			return
		case <-ticker.C:
		}
	}
}

// Update change le message affiché.
func (s *Spinner) Update(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Stop arrête l'animation et retourne la durée écoulée.
func (s *Spinner) Stop() time.Duration {
	if s.animate {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
	<-s.done
	return time.Since(s.start)
}

// Succeed arrête le spinner et affiche une ligne de réussite.
func (s *Spinner) Succeed(msg string) {
	d := s.Stop()
	fmt.Fprintf(Out, "  %s %s %s\n", Green("✔"), msg, Gray(Elapsed(time.Now().Add(-d))))
}

// Skip arrête le spinner et affiche une ligne « déjà configuré ».
func (s *Spinner) Skip(msg string) {
	s.Stop()
	fmt.Fprintf(Out, "  %s %s %s\n", Yellow("↷"), msg, Gray("(déjà configuré)"))
}

// Fail arrête le spinner et affiche une ligne d'échec.
func (s *Spinner) Fail(msg string) {
	s.Stop()
	fmt.Fprintf(Out, "  %s %s\n", Red("✖"), msg)
}

// Elapsed formate une durée écoulée depuis start.
func Elapsed(start time.Time) string {
	d := time.Since(start)
	if d < time.Second {
		return fmt.Sprintf("(%dms)", d.Milliseconds())
	}
	return fmt.Sprintf("(%s)", d.Round(100*time.Millisecond))
}
