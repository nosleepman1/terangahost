package domain

import "errors"

// Sentinel errors typées pour une gestion précise et des conseils d'erreur clairs
var (
	ErrServerNotFound        = errors.New("serveur introuvable dans la configuration locale")
	ErrSiteNotFound          = errors.New("site introuvable")
	ErrServerAlreadyExists   = errors.New("un serveur avec ce nom ou cette adresse IP existe déjà")
	ErrSSHAuthentication     = errors.New("échec d'authentification SSH (vérifiez vos clés ou mot de passe)")
	ErrSSHConnectionTimeout  = errors.New("délai de connexion SSH dépassé (vérifiez l'adresse IP et le port)")
	ErrHostKeyMismatch       = errors.New("l'empreinte de la clé d'hôte SSH a changé (possible attaque MITM)")
	ErrAptLockTimeout        = errors.New("le verrou du gestionnaire de paquets APT/dpkg est resté indisponible trop longtemps")
	ErrUnsupportedOS         = errors.New("système d'exploitation non supporté (Ubuntu 22.04 LTS ou 24.04 LTS requis)")
	ErrDNSPropagationPending = errors.New("le domaine ne pointe pas encore vers l'adresse IP de ce VPS (propagation DNS requise)")
	ErrCommandExecution      = errors.New("erreur lors de l'exécution de la commande distante")
	ErrInsufficientRAM       = errors.New("mémoire RAM insuffisante sur le serveur hôte")
)
