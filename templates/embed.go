package templates

import "embed"

// FS contient l'ensemble des fichiers de configuration compilés directement dans le binaire TerangaHost
//go:embed nginx/* php/* supervisor/* logrotate/*
var FS embed.FS
