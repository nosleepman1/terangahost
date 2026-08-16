# 🇸🇳 TerangaHost

<p align="center">
  <strong>L'Art d'accueillir et propulser vos applications et APIs Laravel en Production sur VPS Ubuntu.</strong><br>
  Outil CLI Open Source en Go, ultra-léger, rapide, sécurisé et pensé pour les développeurs.
</p>

<p align="center">
  <a href="README.md">🇬🇧 Read in English</a> •
  <a href="#-fonctionnalités-clés">Fonctionnalités Clés</a> •
  <a href="#-démarrage-rapide">Démarrage Rapide</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="CONTRIBUTING.md">Contribuer</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Version Go" />
  <img src="https://img.shields.io/badge/Licence-MIT-yellow.svg" alt="Licence" />
  <img src="https://img.shields.io/badge/Plateforme-Ubuntu%2022.04%20%7C%2024.04-E95420?style=flat&logo=ubuntu" alt="Plateforme Ubuntu" />
  <img src="https://img.shields.io/badge/Laravel-10%20%7C%2011%20%7C%2012-FF2D20?style=flat&logo=laravel" alt="Support Laravel" />
</p>

---

## 🌟 Pourquoi TerangaHost ?

Déployer une API Laravel de production sur un VPS nu (*Ubuntu 22.04 / 24.04*) est historiquement complexe, source d'erreurs et coûteux via les panneaux d'hébergement payants :

- **Conflits Nginx & PHP-FPM** : Mauvais calibrage des buffers FastCGI causant des pertes de requêtes sur les gros payloads JSON.
- **Le piège des VPS à 1 Go de RAM** : L'**OOM Killer** de Linux faisant planter MariaDB/MySQL ou gelant les sessions SSH.
- **Queues et Processus d'arrière-plan** : Configuration fastidieuse des daemons Supervisor.
- **Blocages Let's Encrypt** : Dépassement des quotas SSL causé par la latence de propagation DNS.
- **Permissions de fichiers** : Les erreurs 500 récurrentes sur `storage/` et `bootstrap/cache/`.

**TerangaHost** automatise 100 % de cette chaîne dans **un binaire autonome unique**, sans dépendance locale et sans aucune surcharge de RAM sur le VPS.

---

## 🚀 Fonctionnalités Clés

- **⚡ Provisionnement en 1 Commande** : Transforme un VPS Ubuntu brut en serveur de production complet (PHP 8.2 / 8.3 / 8.4 + 14 extensions, Nginx, Supervisor, Composer, MariaDB / PostgreSQL, Redis).
- **🛡️ Sécurité & Moindre Privilège** : Création automatique d'un utilisateur `deployer`, pare-feu UFW (22, 80, 443), Fail2ban, et `sudoers` restreint aux stricts services web.
- **🧠 Protection OOM Killer (Hardware-Aware)** : Création automatique de 2 Go de SWAP (`swappiness=10`) et calcul dynamique des mémoires tampons MySQL et PHP-FPM selon la RAM réelle du serveur.
- **🌐 Isolation Multi-Sites** : Un pool PHP-FPM et des workers Supervisor dédiés par application/API.
- **🔒 SSL Automatique & Pre-flight DNS** : Vérification de la propagation DNS mondiale (1.1.1.1 / 8.8.8.8) avant d'interroger Let's Encrypt pour éviter tout bannissement de quota.
- **🔄 Déploiement Zero-Downtime** : Gestion atomique des releases par liens symboliques, mise en cache automatique (`config:cache`, `route:cache`) et redémarrage fluide des queues.
- **🩺 Diagnostic Intégré (`doctor`)** : Détection instantanée des pannes (disque, RAM, crashs Nginx/PHP).

---

## 📦 Démarrage Rapide

### 1. Compiler depuis les sources
```bash
git clone https://github.com/nosleepman1/terangahost.git
cd terangahost
go build -o bin/terangahost main.go
```

### 2. Provisionner un nouveau serveur VPS
```bash
./bin/terangahost server provision \
  --name=dakar-prod \
  --ip=192.168.1.50 \
  --user=root \
  --ssh-key=~/.ssh/id_ed25519 \
  --php=8.3 \
  --db=mariadb \
  --redis
```

### 3. Créer une API / Site Laravel
```bash
./bin/terangahost site create \
  --server=dakar-prod \
  --domain=api.monprojet.sn \
  --workers=2
```

### 4. Déployer en Zero-Downtime
```bash
./bin/terangahost site deploy \
  --server=dakar-prod \
  --domain=api.monprojet.sn \
  --repo=https://github.com/votre-compte/votre-api-laravel \
  --branch=main
```

### 5. Diagnostiquer la santé du serveur
```bash
./bin/terangahost server doctor --name=dakar-prod
```

---

## 🏛️ Architecture

TerangaHost applique les principes de la **Clean Architecture** en Go :

```text
terangahost/
├── cmd/               # Commandes CLI Cobra (server, site, doctor, list)
├── internal/
│   ├── domain/        # Cœur métier pur (Server, HardwareSpec, Step, Runner, Errors)
│   ├── engine/        # Orchestrateur de Pipeline séquentiel et Idempotence
│   │   └── steps/     # Les 8 étapes de provisionnement système
│   ├── platform/      # Adaptateurs SSH natif (KeepAlive, TOFU), DNS et Storage
│   └── ui/            # Rendu Terminal TUI (Lipgloss, Bubble Tea, Thème Teranga)
├── templates/         # Configurations Nginx, PHP-FPM, Supervisor embarquées (embed.FS)
└── main.go
```

---

## 🤝 Contribution & Communauté

Ce projet est ouvert et maintenu avec cœur pour la communauté tech au Sénégal et en Afrique. Les contributions (Pull Requests, retours d'expérience, optimisations) sont les bienvenues !  
Consultez notre [Guide de Contribution](CONTRIBUTING.md) et notre [Code de Conduite](CODE_OF_CONDUCT.md).

---

## 📄 Licence

Ce projet est distribué sous licence libre [MIT](LICENSE).
