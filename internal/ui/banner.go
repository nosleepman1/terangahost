package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// PrintBanner affiche la bannière textuelle de TerangaHost
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
	fmt.Println(cyan("  Automated Infrastructure Provisioning & Deployment for Laravel"))
	fmt.Println(gray(strings.Repeat("─", 74)))
	fmt.Println()
}
