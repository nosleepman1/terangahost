// Package templates contient les fichiers de configuration compilés dans le binaire.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/nosleepman1/terangahost/internal/shell"
)

// FS contient l'ensemble des modèles de configuration.
//
//go:embed nginx/* php/* supervisor/* logrotate/* laravel/* cron/* deploy/* ssh/* fail2ban/* sudoers/*
var FS embed.FS

var funcs = template.FuncMap{
	"q":    shell.Quote,
	"join": strings.Join,
	"add":  func(a, b int) int { return a + b },
}

// Render exécute le modèle name (chemin relatif dans FS) avec data.
// Les modèles sont du texte brut (text/template) : aucun échappement HTML n'est appliqué.
func Render(name string, data any) ([]byte, error) {
	raw, err := FS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("modèle %s introuvable: %w", name, err)
	}
	tmpl, err := template.New(name).Funcs(funcs).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("modèle %s invalide: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendu du modèle %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// Static retourne un fichier sans substitution.
func Static(name string) ([]byte, error) {
	return FS.ReadFile(name)
}
