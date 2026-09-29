# TerangaHost

<p align="center">
  <strong>Provisionnez un VPS Ubuntu et déployez vos applications Laravel sans interruption, avec un seul binaire.</strong>
</p>

<p align="center">
  <a href="README.md">Read in English</a> •
  <a href="#démarrage-rapide">Démarrage rapide</a> •
  <a href="#commandes">Commandes</a> •
  <a href="#fonctionnement-des-déploiements">Déploiements</a> •
  <a href="#modèle-de-sécurité">Sécurité</a> •
  <a href="CONTRIBUTING.md">Contribuer</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/Licence-MIT-blue.svg" alt="Licence" />
  <img src="https://img.shields.io/badge/Ubuntu-22.04%20%7C%2024.04-E95420?style=flat&logo=ubuntu" alt="Ubuntu" />
  <img src="https://img.shields.io/badge/Laravel-10%20%7C%2011%20%7C%2012-FF2D20?style=flat&logo=laravel" alt="Laravel" />
</p>

---

TerangaHost est une CLI autonome écrite en Go. Elle se connecte à votre serveur en SSH (aucun agent, rien à
installer sur le serveur) et :

1. **provisionne** un VPS Ubuntu 22.04/24.04 vierge pour Laravel ;
2. **crée les sites** : vhost Nginx, pool PHP-FPM dédié, `.env` et base de données générés, Let's Encrypt,
   workers de queue, scheduler, Reverb ;
3. **déploie** votre dépôt Git sans interruption de service, avec retour arrière instantané.

Chaque commande est **idempotente** : relancez-la pour réparer un serveur ou modifier la configuration d'un site.

## Fonctionnalités

- **Provisionnement** : swap de 2 Go, utilisateur `deployer`, UFW (SSH/80/443), Fail2ban, mises à jour de
  sécurité automatiques, PHP 8.2/8.3/8.4 (PPA ondrej, 18 paquets d'extensions, OPcache/JIT), Nginx, Composer
  (signature de l'installeur vérifiée), Supervisor, Certbot, MariaDB ou PostgreSQL (mémoire réglée selon la RAM
  réelle), Redis, durcissement SSH (clés uniquement, appliqué seulement si vous êtes connecté par clé).
- **Sites** : un pool PHP-FPM et un socket Unix par site, budget FPM réparti entre les sites pour ne pas
  saturer la RAM, serveur Nginx par défaut qui rejette les domaines inconnus, en-têtes de sécurité, seul
  `index.php` est exécutable.
- **`.env` généré** : `APP_KEY`, utilisateur de base dédié avec mot de passe aléatoire (transmis sur stdin,
  jamais journalisé), préfixes Redis par site, clés Reverb. Un `.env` existant n'est jamais écrasé.
- **SSL** : vérification DNS préalable auprès de 1.1.1.1 et 8.8.8.8 (A et AAAA) pour éviter les limites de
  Let's Encrypt, validation webroot, renouvellement automatique avec rechargement de Nginx.
- **Déploiements zero-downtime** : clone dans une nouvelle release, `.env`/`storage` partagés, Composer,
  caches config/routes/vues/événements, migrations, bascule atomique du lien `current`, rechargement de PHP-FPM,
  `queue:restart`. **Si une étape échoue avant la bascule, la release est supprimée et la production n'est pas
  touchée.** Deux déploiements simultanés d'un même site sont impossibles.
- **Retour arrière** vers n'importe quelle release conservée, en une commande.
- **`doctor`** : services, configuration Nginx, pare-feu, disque/RAM/charge, unités en échec et, pour chaque
  site : release active, `.env`, expiration du certificat, processus Supervisor, erreurs Laravel.
- **Sûr par défaut** : toutes les entrées sont validées et échappées, les fichiers distants sont écrits de
  façon atomique, les modifications Nginx/PHP-FPM/sudoers/sshd sont validées et annulées si invalides,
  l'inventaire local est écrit atomiquement et jamais écrasé s'il est corrompu.

## Prérequis

- Un VPS **Ubuntu 22.04 ou 24.04** accessible en SSH en `root` ou avec un utilisateur disposant de `sudo` sans mot de passe.
- Une clé SSH (recommandé) chargée dans `ssh-agent` ou passée avec `--ssh-key`.
- Un enregistrement DNS `A` vers le VPS pour activer HTTPS.

## Installation

Téléchargez un binaire depuis la [page des releases](https://github.com/nosleepman1/terangahost/releases), ou compilez-le :

```bash
go install github.com/nosleepman1/terangahost@latest
# ou
git clone https://github.com/nosleepman1/terangahost.git && cd terangahost && make build   # -> bin/terangahost
```

## Démarrage rapide

```bash
# 1. Provisionner le serveur (10 à 15 minutes ; peut être relancé)
terangahost server provision --name=prod --ip=203.0.113.10 --ssh-key=~/.ssh/id_ed25519

# 2. Dépôt privé ? Ajoutez la deploy key du serveur sur GitHub/GitLab (lecture seule)
terangahost server deploy-key --name=prod

# 3. Créer le site (Nginx, PHP-FPM, .env + base, SSL, 1 worker, scheduler)
terangahost site create --server=prod --domain=api.exemple.sn \
  --repo=git@github.com:acme/api.git --branch=main --email=ops@exemple.sn

# 4. Déployer (le dépôt et la branche sont mémorisés)
terangahost site deploy --domain=api.exemple.sn

# Ensuite
terangahost site deploy   --domain=api.exemple.sn
terangahost site rollback --domain=api.exemple.sn
terangahost server doctor --name=prod
```

## Commandes

| Commande | Description |
|---|---|
| `server provision` | Provisionne ou répare un serveur (`--php`, `--db=mariadb\|postgres\|none`, `--redis`, `--user`, `--port`, `--ssh-key`, `--ask-password`, `--harden-ssh`) |
| `server doctor` | Diagnostic du serveur et de ses sites (code de sortie non nul en cas de problème critique) |
| `server list` | Liste les serveurs connus |
| `server deploy-key` | Affiche la clé publique Git du `deployer` |
| `server remove` | Retire un serveur de l'inventaire local (sans toucher au VPS) |
| `server forget-host` | Oublie l'empreinte SSH d'un hôte (après réinstallation du VPS) |
| `site create` | Crée ou met à jour un site (`--alias`, `--php`, `--ssl`, `--email`, `--workers`, `--reverb`, `--reverb-port`, `--scheduler`, `--no-db`, `--repo`, `--branch`, `--skip-dns-check`) |
| `site deploy` | Déploiement zero-downtime (`--repo`, `--branch`, `--no-migrate`, `--keep`, `--timeout`) |
| `site rollback` | Revient à la release précédente (`--release`, `--list`) |
| `site env pull\|push\|edit` | Lit, remplace ou édite dans `$EDITOR` le `.env` de production, puis recharge la configuration |
| `site ssl` | Active HTTPS une fois le DNS en place |
| `site list` | Liste les sites avec leur release et leur commit |
| `site delete` | Retire la configuration d'un site (`--purge` supprime les fichiers, `--drop-database` la base) |

Option globale : `-v/--verbose` affiche la sortie distante. Chaque exécution écrit un journal détaillé dans `~/.terangahost/logs/`.

## Fonctionnement des déploiements

```text
/var/www/api.exemple.sn/
├── current -> releases/20260925120000   # lien atomique servi par Nginx
├── releases/
│   ├── 20260925120000/                  # les 5 dernières releases sont conservées (--keep)
│   └── 20260924093000/
└── shared/
    ├── .env                             # généré une fois, modifiable avec 'site env edit'
    └── storage/                         # stockage persistant (logs, sessions, fichiers)
```

1. `git clone --depth 1` dans `releases/<horodatage>`
2. liaison de `shared/.env` et `shared/storage`
3. `composer install --no-dev --optimize-autoloader`
4. `config:cache`, `route:cache`, `view:cache`, `event:cache`, `storage:link`
5. `migrate --force` (désactivable avec `--no-migrate`)
6. bascule atomique de `current`, rechargement de PHP-FPM, `queue:restart` (et `reverb:restart`)
7. suppression des anciennes releases

Les workers de queue tournent sous Supervisor en tant que `deployer` et exécutent toujours la release active.
Le scheduler est lancé chaque minute depuis `/etc/cron.d`.

## Modèle de sécurité

- Les applications, PHP-FPM, les workers et le scheduler tournent sous l'utilisateur non privilégié **`deployer`**
  (partagé entre les sites, comme Laravel Forge). Chaque site a néanmoins son pool FPM, son socket, ses logs,
  son utilisateur de base de données et son préfixe Redis.
- `deployer` n'a qu'**un** droit sudo : `systemctl reload phpX.Y-fpm` (commandes exactes, sans joker).
  Les règles des anciennes versions de TerangaHost (qui autorisaient `certbot *`, donc root) sont remplacées
  au prochain `server provision`.
- Les commandes d'administration (`provision`, `site create/ssl/delete`, `doctor`) utilisent l'utilisateur
  d'administration ; `deploy`, `rollback` et `env` se connectent en `deployer`.
- Les clés d'hôte SSH sont mémorisées à la première connexion (`~/.terangahost/known_hosts`) ; une clé modifiée
  interrompt la connexion.
- Les secrets (mots de passe de base, contenu du `.env`) passent par stdin et n'apparaissent ni dans la liste des
  processus ni dans les journaux.
- Variables d'environnement pour la CI : `TERANGAHOST_SSH_PASSWORD`, `TERANGAHOST_SSH_PASSPHRASE`, `TERANGAHOST_HOME`.

## Limites

- Ubuntu 22.04/24.04 uniquement.
- Pas encore de build front-end (Node/Vite) pendant le déploiement : versionnez les assets compilés ou compilez-les en CI.
- `rollback` n'annule pas les migrations de base de données.
- Sans Redis, les queues utilisent le driver `database` (la table `jobs` doit exister : c'est le cas par défaut depuis Laravel 11).

## Architecture

```text
cmd/                  commandes Cobra (couche fine : options, affichage, orchestration)
internal/domain/      modèles (Server, Site, HardwareSpec), contrats (Runner, Step, Store), erreurs
internal/engine/      pipeline de provisionnement idempotent, détection matérielle, steps/
internal/site/        configuration des sites (FPM, Nginx, SSL, .env, base, Supervisor, cron)
internal/deploy/      script de déploiement, suivi de progression, releases et rollback
internal/platform/    SSH (agent, TOFU, sudo, envois atomiques), DNS, stockage JSON, journaux
internal/shell|validate  échappement shell et validation des entrées
templates/            fichiers de configuration intégrés au binaire
tests/                tests du pipeline et faux Runner en mémoire
```

## Développement

```bash
make lint test build   # gofmt + go vet, tests avec -race, binaire dans bin/
make build-all         # binaires linux/darwin/windows
```

Le workflow **E2E** provisionne réellement un runner GitHub Ubuntu 24.04 via SSH, crée un site et déploie le
squelette Laravel officiel à chaque push sur `main`.

---

***Auteur : [Abdallah DIOUF](https://github.com/nosleepman1)*** · Distribué sous [licence MIT](LICENSE).
