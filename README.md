# TerangaHost

<p align="center">
  <strong>Provision an Ubuntu VPS and ship Laravel apps with zero downtime, from a single binary.</strong>
</p>

<p align="center">
  <a href="README.fr.md">Lire en français</a> •
  <a href="#quick-start">Quick start</a> •
  <a href="#commands">Commands</a> •
  <a href="#how-deployments-work">Deployments</a> •
  <a href="#security-model">Security</a> •
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" />
  <img src="https://img.shields.io/badge/Ubuntu-22.04%20%7C%2024.04-E95420?style=flat&logo=ubuntu" alt="Ubuntu" />
  <img src="https://img.shields.io/badge/Laravel-10%20%7C%2011%20%7C%2012-FF2D20?style=flat&logo=laravel" alt="Laravel" />
</p>

---

TerangaHost is a standalone CLI written in Go. It connects to your server over SSH (no agent, nothing to
install on the server) and:

1. **provisions** a fresh Ubuntu 22.04/24.04 VPS for Laravel,
2. **creates sites**: Nginx vhost, dedicated PHP-FPM pool, generated `.env` and database, Let's Encrypt,
   queue workers, scheduler, Reverb,
3. **deploys** your Git repository with zero downtime, and rolls back instantly if needed.

Every command is **idempotent**: re-run it to repair a server or change a site's configuration.

## Features

- **Provisioning**: 2 GB swap, `deployer` user, UFW (SSH/80/443), Fail2ban, automatic security updates,
  PHP 8.2/8.3/8.4 (ondrej PPA, 18 extension packages, OPcache/JIT), Nginx, Composer (installer signature
  verified), Supervisor, Certbot, MariaDB or PostgreSQL (memory tuned to the real RAM), Redis, SSH hardening
  (key-only authentication, applied only when you are connected with a key).
- **Sites**: one PHP-FPM pool and Unix socket per site, FPM budget shared between sites to avoid
  overcommitting RAM, a default Nginx server that drops unknown hosts, security headers, only
  `index.php` is executable.
- **Generated `.env`**: `APP_KEY`, a dedicated database user with a random password (sent over stdin, never
  logged), Redis prefixes per site, Reverb keys. An existing `.env` is never overwritten.
- **SSL**: DNS pre-flight check against 1.1.1.1 and 8.8.8.8 (A and AAAA) to avoid Let's Encrypt rate limits,
  webroot validation, automatic renewal with Nginx reload.
- **Zero-downtime deployments**: shallow clone into a new release, shared `.env`/`storage`, Composer,
  config/route/view/event caches, migrations, atomic `current` symlink swap, PHP-FPM reload, graceful
  `queue:restart`. **If any step fails before the swap, the release is deleted and production is untouched.**
  Concurrent deployments of the same site are locked out.
- **Rollback** to any of the retained releases in one command.
- **`doctor`**: services, Nginx config, firewall, disk/RAM/load, failed units, and for each site: active
  release, `.env`, certificate expiry, Supervisor processes, Laravel errors.
- **Safe by default**: all user input is validated and shell-quoted, remote files are written atomically,
  Nginx/PHP-FPM/sudoers/sshd changes are validated and rolled back if invalid, the local inventory is
  written atomically and never overwritten when corrupted.

## Requirements

- A VPS running **Ubuntu 22.04 or 24.04**, reachable over SSH as `root` or as a user with passwordless `sudo`.
- An SSH key (recommended) loaded in `ssh-agent` or passed with `--ssh-key`.
- A DNS `A` record pointing your domain to the VPS if you want HTTPS.

## Installation

Download a binary from the [releases page](https://github.com/nosleepman1/terangahost/releases), or build it:

```bash
go install github.com/nosleepman1/terangahost@latest
# or
git clone https://github.com/nosleepman1/terangahost.git && cd terangahost && make build   # -> bin/terangahost
```

## Quick start

```bash
# 1. Provision the server (10-15 minutes; safe to re-run)
terangahost server provision --name=prod --ip=203.0.113.10 --ssh-key=~/.ssh/id_ed25519

# 2. Private repository? Add the server's deploy key to GitHub/GitLab (read-only)
terangahost server deploy-key --name=prod

# 3. Create the site (Nginx, PHP-FPM, .env + database, SSL, 1 queue worker, scheduler)
terangahost site create --server=prod --domain=api.example.com \
  --repo=git@github.com:acme/api.git --branch=main --email=ops@example.com

# 4. Deploy (repository and branch are remembered)
terangahost site deploy --domain=api.example.com

# Later
terangahost site deploy   --domain=api.example.com
terangahost site rollback --domain=api.example.com
terangahost server doctor --name=prod
```

## Commands

| Command | Description |
|---|---|
| `server provision` | Provision or repair a server (`--php`, `--db=mariadb\|postgres\|none`, `--redis`, `--user`, `--port`, `--ssh-key`, `--ask-password`, `--harden-ssh`) |
| `server doctor` | Health check of the server and its sites (non-zero exit code on critical issues) |
| `server list` | List known servers |
| `server deploy-key` | Print the `deployer` Git public key |
| `server remove` | Remove a server from the local inventory (does not touch the VPS) |
| `server forget-host` | Forget a stored SSH host key (after reinstalling a VPS) |
| `site create` | Create or update a site (`--alias`, `--php`, `--ssl`, `--email`, `--workers`, `--reverb`, `--reverb-port`, `--scheduler`, `--no-db`, `--repo`, `--branch`, `--skip-dns-check`) |
| `site deploy` | Zero-downtime deployment (`--repo`, `--branch`, `--no-migrate`, `--keep`, `--timeout`) |
| `site rollback` | Switch back to the previous release (`--release`, `--list`) |
| `site env pull\|push\|edit` | Read, replace, or edit the production `.env` in `$EDITOR`, then reload config |
| `site ssl` | Enable HTTPS once DNS points to the server |
| `site list` | List sites with their current release and commit |
| `site delete` | Remove a site's configuration (`--purge` deletes files, `--drop-database` drops the DB) |

Global flag: `-v/--verbose` streams the remote output. Every run writes a detailed log to `~/.terangahost/logs/`.

## How deployments work

```text
/var/www/api.example.com/
├── current -> releases/20260925120000   # atomic symlink served by Nginx
├── releases/
│   ├── 20260925120000/                  # the 5 most recent releases are kept (--keep)
│   └── 20260924093000/
└── shared/
    ├── .env                             # generated once, edited with 'site env edit'
    └── storage/                         # persistent storage (logs, sessions, uploads)
```

1. `git clone --depth 1` into `releases/<timestamp>`
2. link `shared/.env` and `shared/storage`
3. `composer install --no-dev --optimize-autoloader`
4. `config:cache`, `route:cache`, `view:cache`, `event:cache`, `storage:link`
5. `migrate --force` (skip with `--no-migrate`)
6. atomic swap of `current`, PHP-FPM reload, `queue:restart` (and `reverb:restart`)
7. old releases are pruned

Queue workers run under Supervisor as `deployer` and always execute the active release. The scheduler runs
every minute from `/etc/cron.d`.

## Security model

- Applications, PHP-FPM, workers and the scheduler run as the unprivileged **`deployer`** user (shared by all
  sites, as with Laravel Forge). Each site still gets its own FPM pool, socket, logs, database user and Redis prefix.
- `deployer` has **one** sudo right: `systemctl reload phpX.Y-fpm` (exact commands, no wildcards).
  Rules from older versions of TerangaHost (which allowed `certbot *` and therefore root) are replaced
  on the next `server provision`.
- Administrative commands (`provision`, `site create/ssl/delete`, `doctor`) connect with the admin user;
  `deploy`, `rollback` and `env` connect as `deployer`.
- SSH host keys are pinned on first use (`~/.terangahost/known_hosts`); a changed key aborts the connection.
- Secrets (DB passwords, `.env` content) are sent over stdin and never appear in process lists or logs.
- Environment variables for CI: `TERANGAHOST_SSH_PASSWORD`, `TERANGAHOST_SSH_PASSPHRASE`, `TERANGAHOST_HOME`.

## Limitations

- Ubuntu 22.04/24.04 only.
- No front-end build step (Node/Vite) during deployment yet: commit built assets or build them in CI.
- `rollback` does not revert database migrations.
- Without Redis, queues use the `database` driver (the `jobs` table must exist: default since Laravel 11).

## Architecture

```text
cmd/                  Cobra commands (thin layer: flags, output, orchestration)
internal/domain/      models (Server, Site, HardwareSpec), contracts (Runner, Step, Store), errors
internal/engine/      idempotent provisioning pipeline, hardware detection, steps/
internal/site/        site configuration (FPM, Nginx, SSL, .env, database, Supervisor, cron)
internal/deploy/      deployment script, progress parsing, releases and rollback
internal/platform/    SSH (agent, TOFU, sudo, atomic uploads), DNS, JSON storage, logs
internal/shell|validate  shell quoting and input validation
templates/            configuration files embedded in the binary
tests/                pipeline tests and the in-memory Runner mock
```

## Development

```bash
make lint test build   # gofmt + go vet, tests with -race, binary in bin/
make build-all         # linux/darwin/windows binaries
```

The **E2E** workflow provisions a real Ubuntu 24.04 GitHub runner over SSH, creates a site and deploys the
official Laravel skeleton on every push to `main`.

---

***Author: [Abdallah DIOUF](https://github.com/nosleepman1)*** · Licensed under the [MIT License](LICENSE).
