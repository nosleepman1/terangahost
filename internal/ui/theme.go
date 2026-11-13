// Package ui regroupe l'affichage terminal : couleurs, messages et indicateurs de progression.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/term"
)

// Out est la destination de l'affichage (remplaçable dans les tests).
var Out io.Writer = os.Stdout

var (
	Cyan   = color.New(color.FgCyan, color.Bold).SprintFunc()
	Green  = color.New(color.FgGreen, color.Bold).SprintFunc()
	Yellow = color.New(color.FgYellow, color.Bold).SprintFunc()
	Red    = color.New(color.FgRed, color.Bold).SprintFunc()
	Gold   = color.New(color.FgHiYellow, color.Bold).SprintFunc()
	Gray   = color.New(color.FgHiBlack).SprintFunc()
)

// IsTTY indique si la sortie standard est un terminal interactif.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Info affiche une ligne d'information.
func Info(format string, args ...any) {
	fmt.Fprintf(Out, format+"\n", args...)
}

// Success affiche un message de réussite.
func Success(format string, args ...any) {
	fmt.Fprintf(Out, "%s %s\n", Green("✔"), fmt.Sprintf(format, args...))
}

// Warn affiche un avertissement.
func Warn(format string, args ...any) {
	fmt.Fprintf(Out, "%s %s\n", Yellow("!"), fmt.Sprintf(format, args...))
}

// Fail affiche une erreur non fatale.
func Fail(format string, args ...any) {
	fmt.Fprintf(Out, "%s %s\n", Red("✖"), fmt.Sprintf(format, args...))
}

// Hint affiche une commande suggérée.
func Hint(cmd string) {
	fmt.Fprintf(Out, "    %s\n", Cyan(cmd))
}

// Section affiche un titre de section.
func Section(title string) {
	fmt.Fprintf(Out, "\n%s\n", Cyan(title))
}

// Rule affiche un séparateur horizontal.
func Rule() {
	fmt.Fprintln(Out, Gray(strings.Repeat("─", 74)))
}

// KV affiche une paire clé/valeur alignée.
func KV(key, value string) {
	fmt.Fprintf(Out, "  %-22s %s\n", key+" :", value)
}
