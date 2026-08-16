package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Palette de couleurs inspirée de l'Afrique & de l'esprit Teranga (Or / Ambre, Vert forêt, Cyan, Bleu nuit)
	ColorPrimary   = lipgloss.Color("#E5A93C") // Ambre doré (Teranga Gold)
	ColorSecondary = lipgloss.Color("#10B981") // Vert émeraude / succès
	ColorAccent    = lipgloss.Color("#06B6D4") // Cyan
	ColorMuted     = lipgloss.Color("#64748B") // Gris ardoise
	ColorDanger    = lipgloss.Color("#EF4444") // Rouge erreur
	ColorWarning   = lipgloss.Color("#F59E0B") // Ambre alerte

	// Styles de texte et de boîtes
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	BadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorPrimary).
			Padding(0, 1)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorDanger)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)
)
