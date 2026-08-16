package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// PrintBanner affiche la bannière officielle de TerangaHost
func PrintBanner() {
	gold := color.New(color.FgHiYellow, color.Bold).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	gray := color.New(color.FgHiBlack).SprintFunc()

	banner := `
  ████████╗███████╗██████╗  █████╗ ███╗   ██╗ ██████╗  █████╗ 
  ╚══██╔══╝██╔════╝██╔══██╗██╔══██╗████╗  ██║██╔════╝ ██╔══██╗
     ██║   █████╗  ██████╔╝███████║██╔██╗ ██║██║  ███╗███████║
     ██║   ██╔══╝  ██╔══██╗██╔══██║██║╚██╗██║██║   ██║██╔══██║
     ██║   ███████╗██║  ██║██║  ██║██║ ╚████║╚██████╔╝██║  ██║
     ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝ ╚═╝  ╚═╝
     ██╗  ██╗ ██████╗ ███████╗████████╗                       
     ██║  ██║██╔═══██╗██╔════╝╚══██╔══╝                       
     ███████║██║   ██║███████╗   ██║                          
     ██╔══██║██║   ██║╚════██║   ██║                          
     ██║  ██║╚██████╔╝███████║   ██║                          
     ╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝                          `

	fmt.Println(gold(banner))
	fmt.Println(cyan("  🇸🇳  L'Art d'accueillir et propulser vos APIs Laravel en Production"))
	fmt.Println(gray(strings.Repeat("─", 74)))
	fmt.Println()
}
