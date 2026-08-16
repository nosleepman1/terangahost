# 🇸🇳 TerangaHost

> **L'art d'accueillir et propulser vos applications et APIs Laravel en Production sur VPS Ubuntu.**  
> Outil CLI Open Source en Go, ultra-léger, rapide, sécurisé et pensé pour les développeurs.

---

## 🌟 Pourquoi TerangaHost ?

Déployer et configurer une API Laravel de production sur un VPS nu (*Ubuntu 22.04 / 24.04*) est historiquement chronophage et source de pannes (conflits Nginx / PHP-FPM, crashs OOM sur les VPS 1 Go, erreurs 500 sur les permissions `storage/`, daemons Supervisor, blocages Let's Encrypt).

**TerangaHost** automatise 100 % de cette chaîne sans surcoût, sans interface lourde consommatrice de RAM, et sous forme d'un **binaire unique sans dépendance**.

---

## 🚀 Fonctionnalités Clés

- **⚡ Provisionnement en 1 Commande** : Transforme un VPS Ubuntu brut en serveur de production complet (PHP 8.2/8.3/8.4 + 14 extensions, Nginx, Supervisor, Composer, MariaDB/PostgreSQL, Redis).
- **🛡️ Sécurité & Moindre Privilège** : Création automatique d'un utilisateur `deployer`, pare-feu UFW (22, 80, 443), Fail2ban, et `sudoers` restreint aux stricts services web.
- **🧠 Protection OOM Killer (Hardware-Aware)** : Création automatique de 2 Go de SWAP et calcul dynamique des mémoires tampons MySQL et PHP-FPM selon la RAM réelle du serveur.
- **🌐 Isolation Multi-Sites** : Un pool PHP-FPM et des workers Supervisor dédiés par application/API.
- **🔒 SSL Automatique & Pre-flight DNS** : Vérification de la propagation DNS avant d'interroger Let's Encrypt pour éviter tout bannissement de quota.
- **🔄 Déploiement Zero-Downtime** : Gestion atomique des releases par liens symboliques, mise en cache automatique (`config:cache`, `route:cache`) et redémarrage fluide des queues.
- **🩺 Diagnostic Intégré (`doctor`)** : Détection instantanée des pannes (disque, RAM, crashs Nginx/PHP).

---

## 📦 Installation & Démarrage Rapide

### 1. Cloner et compiler
```bash
git clone https://github.com/teranga-host/terangahost.git
cd terangahost
go build -o terangahost main.go
```

### 2. Provisionner un nouveau serveur VPS
```bash
terangahost server provision \
  --name=dakar-prod-01 \
  --ip=192.168.1.50 \
  --user=root \
  --ssh-key=~/.ssh/id_ed25519 \
  --php=8.3 \
  --db=mariadb \
  --redis
```

### 3. Créer un site / API Laravel
```bash
terangahost site create \
  --server=dakar-prod-01 \
  --domain=api.monprojet.sn \
  --workers=2
```

### 4. Déployer en Zero-Downtime
```bash
terangahost site deploy \
  --server=dakar-prod-01 \
  --domain=api.monprojet.sn \
  --repo=https://github.com/votre-compte/votre-api-laravel \
  --branch=main
```

### 5. Inspecter la santé du serveur
```bash
terangahost server doctor --name=dakar-prod-01
```

---

## 🏛️ Architecture Technique

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
