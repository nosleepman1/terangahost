# TerangaHost

<p align="center">
  <strong>Outil CLI d'automatisation d'infrastructure et de deploiement zero-downtime pour APIs Laravel sur VPS Ubuntu.</strong>
</p>

<p align="center">
  <a href="README.md">Read in English</a> •
  <a href="#fonctionnalites-principales">Fonctionnalites</a> •
  <a href="#demarrage-rapide">Demarrage Rapide</a> •
  <a href="#architecture">Architecture</a> •
  <a href="CONTRIBUTING.md">Contribution</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Version Go" />
  <img src="https://img.shields.io/badge/Licence-MIT-blue.svg" alt="Licence" />
  <img src="https://img.shields.io/badge/Plateforme-Ubuntu%2022.04%20%7C%2024.04-E95420?style=flat&logo=ubuntu" alt="Plateforme Ubuntu" />
  <img src="https://img.shields.io/badge/Laravel-10%20%7C%2011%20%7C%2012-FF2D20?style=flat&logo=laravel" alt="Support Laravel" />
</p>

---

## Presentation

Le deploiement et l'administration d'applications et d'APIs Laravel de production sur des instances VPS Ubuntu brutes presentent des difficultes operationnelles recurrentes :

- **Conflits Nginx et PHP-FPM** : Dimensionnement inadapte des tampons FastCGI generant des pertes de requetes sur les charges JSON elevees.
- **Instabilite sur VPS a memoire limitee (1 Go RAM)** : Declenchement du processus OOM Killer de Linux provoquant l'arret force de la base de donnees lors des operations Composer.
- **Gestion des processus d'arriere-plan** : Configuration complexe des services Supervisor et gestion des signaux d'arret.
- **Limites de requetes ACME** : Echecs de generation de certificats Let's Encrypt dus aux delais de propagation DNS.
- **Gestion des permissions** : Conflits de droits d'acces sur les repertoires de stockage et de cache.

**TerangaHost** est un outil en ligne de commande compile en Go, autonome et sans dependance locale, qui automatise le provisionnement, le durcissement de la securite, l'isolation multi-sites et les deploiements zero-downtime sans surconsommation de ressources sur le serveur distant.

---

## Fonctionnalites Principales

- **Provisionnement automatise en une commande** : Configuration complete d'instances Ubuntu 22.04 et 24.04 LTS incluant PHP (8.2, 8.3, 8.4) et ses 14 extensions indispensables, Nginx, Supervisor, Composer, MariaDB ou PostgreSQL, et Redis.
- **Modele de securite au moindre privilege** : Creation d'un utilisateur dedie `deployer`, regles de pare-feu UFW strictes (ports 22, 80, 443), configuration de Fail2ban et restriction fine des droits `sudoers` aux seuls services requis.
- **Tuning adaptatif selon le materiel (Hardware-Aware)** : Creation automatique de 2 Go de swap (`swappiness=10`) et calcul dynamique de la taille du pool InnoDB et des processus PHP-FPM selon la RAM reelle.
- **Isolation multi-sites** : Sockets Unix et pools de processus PHP-FPM dedies par application pour eviter tout risque d'epuisement transversal des ressources.
- **Verification DNS pre-vol** : Requetes directes aupres des serveurs DNS racines (1.1.1.1 et 8.8.8.8) afin de verifier la resolution du domaine avant toute demande de certificat SSL.
- **Deploiements atomiques sans interruption (Zero-Downtime)** : Bascule par liens symboliques, generation automatique des caches de production et redemarrage ordonne des files d'attente.
- **Diagnostic integre (`doctor`)** : Verification immediate de l'utilisation disque, de la memoire disponible, de l'etat des services et des logs d'erreur recents.

---

## Demarrage Rapide

### 1. Compilation depuis les sources
```bash
git clone https://github.com/nosleepman1/terangahost.git
cd terangahost
go build -o bin/terangahost main.go
```

### 2. Provisionner un serveur VPS
```bash
./bin/terangahost server provision \
  --name=prod-01 \
  --ip=192.168.1.50 \
  --user=root \
  --ssh-key=~/.ssh/id_ed25519 \
  --php=8.3 \
  --db=mariadb \
  --redis
```

### 3. Configurer un site / API Laravel
```bash
./bin/terangahost site create \
  --server=prod-01 \
  --domain=api.domaine.sn \
  --workers=2
```

### 4. Deployer le code applicatif
```bash
./bin/terangahost site deploy \
  --server=prod-01 \
  --domain=api.domaine.sn \
  --repo=https://github.com/votre-organisation/votre-api-laravel \
  --branch=main
```

### 5. Executer le diagnostic de sante
```bash
./bin/terangahost server doctor --name=prod-01
```

---

## Architecture

TerangaHost applique les principes de la Clean Architecture (Ports & Adaptateurs) :

```text
terangahost/
├── cmd/               # Couche CLI (Commandes Cobra: server, site, doctor, list)
├── internal/
│   ├── domain/        # Modeles metier (Server, HardwareSpec, Step, Runner, Errors)
│   ├── engine/        # Moteur d'orchestration, tuner adaptatif et etapes
│   ├── platform/      # Client SSH natif (KeepAlive, TOFU), DNS et persistance
│   └── ui/            # Formats de sortie et presentation terminale
├── templates/         # Configurations embarquees Nginx, PHP-FPM, Supervisor et Logrotate
└── main.go
```

---

## Contribution

Veuillez consulter notre [Guide de Contribution](CONTRIBUTING.md) et notre [Code de Conduite](CODE_OF_CONDUCT.md) avant toute soumission de contribution.

---

## Licence

Ce projet est distribue sous licence libre [MIT](LICENSE).
