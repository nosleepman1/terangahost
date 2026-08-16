# 🇸🇳 TerangaHost

<p align="center">
  <strong>The Art of Hosting and Scaling Production Laravel APIs on Ubuntu VPS.</strong><br>
  An ultra-fast, zero-dependency, open-source Go CLI tool built for developers.
</p>

<p align="center">
  <a href="README.fr.md">🇫🇷 Lire en Français</a> •
  <a href="#-key-features">Key Features</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License" />
  <img src="https://img.shields.io/badge/Platform-Ubuntu%2022.04%20%7C%2024.04-E95420?style=flat&logo=ubuntu" alt="Ubuntu Platform" />
  <img src="https://img.shields.io/badge/Laravel-10%20%7C%2011%20%7C%2012-FF2D20?style=flat&logo=laravel" alt="Laravel Support" />
</p>

---

## 🌟 Why TerangaHost?

Deploying a production Laravel API on a bare-metal **Ubuntu VPS** is notoriously complex, prone to misconfigurations, and expensive when using hosted management panels:

- **Nginx & PHP-FPM Socket Conflicts**: Suboptimal FastCGI buffer sizes causing API request drops.
- **Low RAM VPS Crisis (1GB RAM)**: The Linux **OOM Killer** crashing MySQL or hanging SSH sessions during deployments.
- **Background Workers & Queues**: Tedious Supervisor daemon configuration and restart policies.
- **SSL Rate Limits**: Let's Encrypt domain bans due to DNS propagation delays.
- **Permission Pitfalls**: Infamous 500 errors on `storage/` and `bootstrap/cache/`.

**TerangaHost** automates the entire infrastructure lifecycle in **a single, standalone binary** with zero local dependencies and zero VPS background bloat.

---

## 🚀 Key Features

- **⚡ 1-Command Server Provisioning**: Turns a fresh Ubuntu 22.04 / 24.04 VPS into an enterprise-grade Laravel production server (PHP 8.2 / 8.3 / 8.4 + 14 extensions, Nginx, Supervisor, Composer, MariaDB / PostgreSQL, Redis).
- **🛡️ Least-Privilege Security**: Creates an isolated `deployer` user, strictly configures UFW firewall (ports 22, 80, 443), enables Fail2ban, and scopes `sudoers` to essential web services only.
- **🧠 Hardware-Aware Memory Tuning**: Automatically creates 2GB of SWAP (`swappiness=10`) and dynamically calculates MySQL buffer pools and PHP-FPM process limits based on physical RAM.
- **🌐 Multi-Tenant Site Isolation**: Dedicated PHP-FPM pools and namespaced Supervisor queue workers per application.
- **🔒 Pre-Flight DNS & Auto SSL**: Queries global DNS (1.1.1.1 / 8.8.8.8) to verify domain resolution before requesting Let's Encrypt certificates, preventing rate-limit bans.
- **🔄 Zero-Downtime Atomic Releases**: Symlink-based release pipeline with automated caching (`config:cache`, `route:cache`, `view:cache`) and graceful queue restarts.
- **🩺 Instant Health Diagnostics (`doctor`)**: Inspects RAM, disk, service daemons, and recent Nginx/Laravel error logs in under 3 seconds.

---

## 📦 Quick Start

### 1. Build from Source
```bash
git clone https://github.com/nosleepman1/terangahost.git
cd terangahost
go build -o bin/terangahost main.go
```

### 2. Provision a New VPS
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

### 3. Create a Laravel Site / API
```bash
./bin/terangahost site create \
  --server=dakar-prod \
  --domain=api.myapp.sn \
  --workers=2
```

### 4. Deploy with Zero-Downtime
```bash
./bin/terangahost site deploy \
  --server=dakar-prod \
  --domain=api.myapp.sn \
  --repo=https://github.com/your-org/your-laravel-api \
  --branch=main
```

### 5. Inspect Server Health
```bash
./bin/terangahost server doctor --name=dakar-prod
```

---

## 🏛️ Architecture

TerangaHost is engineered using **Clean / Ports & Adapters Architecture** in Go:

```text
terangahost/
├── cmd/               # Cobra CLI commands (server, site, doctor, list)
├── internal/
│   ├── domain/        # Core business models (Server, HardwareSpec, Step, Runner)
│   ├── engine/        # Sequential pipeline engine, hardware-aware tuner, steps
│   ├── platform/      # Native Go SSH client (KeepAlive, TOFU), DNS & storage
│   └── ui/            # Lipgloss / Bubble Tea terminal presentation
├── templates/         # Embedded Nginx, PHP-FPM, Supervisor & Logrotate configs
└── main.go
```

---

## 🤝 Contributing

We welcome contributions from developers worldwide, with a special focus on empowering the tech ecosystem in Senegal and Africa!  
Please read our [Contributing Guide](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md).

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).
