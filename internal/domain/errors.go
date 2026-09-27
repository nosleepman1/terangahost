package domain

import "errors"

// Erreurs sentinelles typées : utilisées avec errors.Is pour afficher des conseils précis.
var (
	ErrServerNotFound        = errors.New("serveur introuvable dans la configuration locale")
	ErrSiteNotFound          = errors.New("site introuvable dans la configuration locale")
	ErrServerAlreadyExists   = errors.New("un serveur avec ce nom existe déjà avec une autre adresse IP")
	ErrSiteAlreadyExists     = errors.New("ce domaine est déjà configuré sur un autre serveur")
	ErrInvalidInput          = errors.New("paramètre invalide")
	ErrConfigCorrupted       = errors.New("fichier de configuration local illisible")
	ErrSSHAuthentication     = errors.New("échec d'authentification SSH (vérifiez vos clés ou mot de passe)")
	ErrSSHConnectionTimeout  = errors.New("connexion SSH impossible (vérifiez l'adresse IP, le port et le pare-feu)")
	ErrHostKeyMismatch       = errors.New("l'empreinte de la clé d'hôte SSH a changé (possible attaque MITM)")
	ErrSudoRequired          = errors.New("l'utilisateur SSH doit être root ou disposer de sudo sans mot de passe")
	ErrAptLockTimeout        = errors.New("le verrou du gestionnaire de paquets APT/dpkg est resté indisponible trop longtemps")
	ErrUnsupportedOS         = errors.New("système d'exploitation non supporté (Ubuntu 22.04 LTS ou 24.04 LTS requis)")
	ErrDNSPropagationPending = errors.New("le domaine ne pointe pas encore vers l'adresse IP de ce VPS (propagation DNS requise)")
	ErrCommandExecution      = errors.New("erreur lors de l'exécution de la commande distante")
)
