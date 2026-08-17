# TerangaHost

<p align="center">
  <strong>High-Performance Infrastructure Provisioner and Zero-Downtime Deployment CLI for Laravel APIs on Ubuntu VPS.</strong>
</p>

<p align="center">
  <a href="README.fr.md">Lire en Francais</a> •
  <a href="#key-features">Key Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#architecture">Architecture</a> •
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License" />
  <img src="https://img.shields.io/badge/Platform-Ubuntu%2022.04%20%7C%2024.04-E95420?style=flat&logo=ubuntu" alt="Ubuntu Platform" />
  <img src="https://img.shields.io/badge/Laravel-10%20%7C%2011%20%7C%2012-FF2D20?style=flat&logo=laravel" alt="Laravel Support" />
</p>

---

## Overview

Deploying and managing production-grade Laravel APIs on bare-metal Ubuntu VPS instances presents persistent operational challenges:

- **Nginx and PHP-FPM Socket Contention**: Inappropriate FastCGI buffer sizes causing dropped connections on high-throughput JSON workloads.
- **Low-Memory VPS Degradation**: Linux Out-Of-Memory (OOM) killer terminating database processes during Composer installations on 1GB RAM instances.
- **Background Worker Orchestration**: Complex Supervisor daemon configuration, signal management, and restart strategies.
- **ACME Rate-Limiting**: Let's Encrypt domain validation failures caused by asynchronous DNS propagation.
- **Permission Discrepancies**: File permission conflicts across storage and cache directories.

**TerangaHost** is a compiled, standalone CLI tool written in Go that automates server provisioning, security hardening, multi-tenant isolation, and zero-downtime deployments without local dependencies or runtime overhead on target instances.

---

## Key Features

- **Single-Command Server Provisioning**: Automated configuration of Ubuntu 22.04 and 24.04 LTS servers including PHP (8.2, 8.3, 8.4) with all 14 mandatory extensions, Nginx, Supervisor, Composer, MariaDB or PostgreSQL, and Redis.
- **Least-Privilege Security Model**: Automated creation of a dedicated `deployer` user, strict UFW firewall rules (ports 22, 80, 443), Fail2ban daemon configuration, and granular `sudoers` policies restricted strictly to web services.
- **Hardware-Aware Memory Tuning**: Automatic 2GB swap partition allocation (`swappiness=10`) with dynamic calculation of InnoDB buffer pool sizes and PHP-FPM process limits based on real physical memory.
- **Multi-Tenant Site Isolation**: Dedicated Unix sockets and PHP-FPM process pools per application to prevent cross-site resource exhaustion.
- **Pre-Flight DNS Verification**: Direct upstream DNS queries (1.1.1.1 and 8.8.8.8) to validate domain propagation prior to triggering ACME / Let's Encrypt issuance.
- **Atomic Zero-Downtime Deployments**: Symlink-switched release pipeline with automated framework caching and background worker reloading.
- **System Health Diagnostics (`doctor`)**: Real-time inspection of disk usage, RAM utilization, active daemons, and recent Nginx / Laravel error logs.

---

## Quick Start

### 1. Build from Source
```bash
git clone https://github.com/nosleepman1/terangahost.git
cd terangahost
go build -o bin/terangahost main.go
```

### 2. Provision a New Server
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

### 3. Configure a Laravel Site / API
```bash
./bin/terangahost site create \
  --server=prod-01 \
  --domain=api.domain.com \
  --workers=2
```

### 4. Deploy Application Code
```bash
./bin/terangahost site deploy \
  --server=prod-01 \
  --domain=api.domain.com \
  --repo=https://github.com/your-org/your-laravel-api \
  --branch=main
```

### 5. Run System Diagnostics
```bash
./bin/terangahost server doctor --name=prod-01
```

---

## Architecture

TerangaHost implements Clean Architecture (Ports & Adapters) principles:

```text
terangahost/
├── cmd/               # CLI layer (Cobra commands: server, site, doctor, list)
├── internal/
│   ├── domain/        # Core business models (Server, HardwareSpec, Step, Runner, Errors)
│   ├── engine/        # Sequential pipeline engine, hardware-aware tuner, steps
│   ├── platform/      # Native Go SSH client (KeepAlive, TOFU), DNS & storage
│   └── ui/            # Terminal formatting and output renderers
├── templates/         # Embedded Nginx, PHP-FPM, Supervisor, and Logrotate configs
└── main.go
```


---

## Contributing

Please review our [Contributing Guidelines](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before submitting pull requests.

---

***Author [Abdallah DIOUF](https://github.com/nosleepman1)***

---

## License

This project is licensed under the [MIT License](LICENSE).
