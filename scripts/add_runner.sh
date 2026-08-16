#!/usr/bin/env bash
set -euo pipefail

RUNNER_VERSION="2.319.1"
ARCHIVE_NAME="actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
CACHE_DIR="${HOME}/.cache/github-runner"
RUNNERS_BASE_DIR="${HOME}/runners"

echo "=================================================================="
echo " TerangaHost - GitHub Self-Hosted Runner Provisioner"
echo "=================================================================="
echo ""

# 1. Demande du nom du runner / projet
read -r -p "[INPUT] Nom du projet / runner (ex: api-laravel) : " RUNNER_NAME
if [[ -z "${RUNNER_NAME}" ]]; then
    echo "[ERROR] Le nom du runner ne peut pas etre vide."
    exit 1
fi

TARGET_DIR="${RUNNERS_BASE_DIR}/${RUNNER_NAME}"
if [[ -d "${TARGET_DIR}" ]]; then
    echo "[WARN] Le dossier ${TARGET_DIR} existe deja."
    read -r -p "[INPUT] Voulez-vous le remplacer ? (y/N) : " OVERWRITE
    if [[ "${OVERWRITE}" != "y" && "${OVERWRITE}" != "Y" ]]; then
        echo "[ABORT] Operation annulee."
        exit 0
    fi
    cd "${TARGET_DIR}" && sudo ./svc.sh stop 2>/dev/null || true
    cd "${TARGET_DIR}" && sudo ./svc.sh uninstall 2>/dev/null || true
    rm -rf "${TARGET_DIR}"
fi

# 2. Demande de l'URL du repository
read -r -p "[INPUT] URL du depot GitHub (ex: https://github.com/user/repo) : " REPO_URL
if [[ -z "${REPO_URL}" ]]; then
    echo "[ERROR] L'URL du depot est requise."
    exit 1
fi

# 3. Demande du Token GitHub
read -r -p "[INPUT] Token GitHub Runner : " RUNNER_TOKEN
if [[ -z "${RUNNER_TOKEN}" ]]; then
    echo "[ERROR] Le token est requis."
    exit 1
fi

# 4. Preparation du cache et telechargement si necessaire
mkdir -p "${CACHE_DIR}"
if [[ ! -f "${CACHE_DIR}/${ARCHIVE_NAME}" ]]; then
    echo "[INFO] Telechargement de l'agent GitHub Runner v${RUNNER_VERSION}..."
    curl -sL -o "${CACHE_DIR}/${ARCHIVE_NAME}" "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${ARCHIVE_NAME}"
    echo "[OK] Telechargement termine."
else
    echo "[INFO] Archive locale detectee dans le cache."
fi

# 5. Extraction dans le dossier dedie
mkdir -p "${TARGET_DIR}"
echo "[INFO] Extraction de l'agent dans ${TARGET_DIR}..."
tar xzf "${CACHE_DIR}/${ARCHIVE_NAME}" -C "${TARGET_DIR}"
cd "${TARGET_DIR}"

# 6. Configuration non-interactive
echo "[INFO] Enregistrement du runner aupres de GitHub..."
./config.sh \
    --url "${REPO_URL}" \
    --token "${RUNNER_TOKEN}" \
    --name "${RUNNER_NAME}" \
    --work "_work" \
    --unattended \
    --replace

# 7. Installation et demarrage du service Systemd
echo "[INFO] Configuration du service d'arriere-plan Systemd..."
sudo ./svc.sh install "${USER}"
sudo ./svc.sh start

echo ""
echo "=================================================================="
echo "[SUCCESS] Runner '${RUNNER_NAME}' installe et actif avec succes !"
echo "Dossier : ${TARGET_DIR}"
echo "Statut  : En ecoute de jobs (Background Service)"
echo "=================================================================="
