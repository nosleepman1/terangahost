package ui

import (
	"fmt"
	"strings"
)

// PrintBanner affiche la bannière de TerangaHost (uniquement dans un terminal interactif).
func PrintBanner(version string) {
	if !IsTTY() {
		return
	}
	banner := `
  ████████╗███████╗██████╗  █████╗ ███╗   ██╗ ██████╗  █████╗
  ╚══██╔══╝██╔════╝██╔══██╗██╔══██╗████╗  ██║██╔════╝ ██╔══██╗
     ██║   █████╗  ██████╔╝███████║██╔██╗ ██║██║  ███╗███████║
     ██║   ██╔══╝  ██╔══██╗██╔══██║██║╚██╗██║██║   ██║██╔══██║
     ██║   ███████╗██║  ██║██║  ██║██║ ╚████║╚██████╔╝██║  ██║
     ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝ ╚═╝  ╚═╝
                         H O S T`
	fmt.Fprintln(Out, Gold(banner))
	fmt.Fprintf(Out, "  %s %s\n", Cyan("Provisioning & zero-downtime deployment for Laravel"), Gray(version))
	fmt.Fprintln(Out, Gray(strings.Repeat("─", 74)))
}
