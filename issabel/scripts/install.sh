#!/usr/bin/env bash
#
# VoiceOTP Gateway - Issabel installation script
#
# Installs the VoiceOTP Gateway Go service, the dedicated Asterisk AMI user,
# the isolated dialplan context, and the systemd unit on an Issabel server.
#
# Idempotent: safe to re-run. Never overwrites or deletes existing PBX
# configuration outside of files created by this project.
#
# Usage: sudo ./issabel/scripts/install.sh [config.json]
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BINARY_NAME="voiceotp-gateway"
INSTALL_BIN="/usr/local/bin/${BINARY_NAME}"
CONFIG_DIR="/etc/voiceotp"
TLS_DIR="${CONFIG_DIR}/tls"
DATA_DIR="/var/lib/voiceotp"
LOG_DIR="/var/log/voiceotp"
RUN_DIR="/var/run/voiceotp"
SERVICE_USER="voiceotp"
SERVICE_GROUP="voiceotp"
SYSTEMD_UNIT="/etc/systemd/system/voiceotp.service"
UNIT_SOURCE="${PROJECT_ROOT}/issabel/systemd/voiceotp.service"
ASTERISK_ETC="${ASTERISK_ETC:-/etc/asterisk}"
ASTERISK_VAR="${ASTERISK_VAR:-/var/lib/asterisk}"
CONFIG_SOURCE="${1:-}"
CONFIG_DEST="${CONFIG_DIR}/config.json"
CONFIG_EXAMPLE="${PROJECT_ROOT}/issabel/config/config.example.json"
ASTERISK_CONF="${ASTERISK_ETC}/asterisk.conf"
MANAGER_CONF="${ASTERISK_ETC}/manager.conf"
EXTENSIONS_CONF="${ASTERISK_ETC}/extensions.conf"
DIALPLAN_FILE="${ASTERISK_ETC}/voiceotp-playback.conf"
AMI_FILE="${ASTERISK_ETC}/voiceotp_ami.conf"
AMI_EXAMPLE="${PROJECT_ROOT}/issabel/config/voiceotp_ami.conf.example"

log() { printf '[install] %s\n' "$*"; }
err() { printf '[install][ERROR] %s\n' "$*" >&2; exit 1; }

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    err "This script must be run as root (use sudo)."
  fi
}

detect_os() {
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS_NAME="${ID}"
    OS_VERSION="${VERSION_ID}"
  elif command -v lsb_release >/dev/null 2>&1; then
    OS_NAME="$(lsb_release -si | tr '[:upper:]' '[:lower:]')"
    OS_VERSION="$(lsb_release -sr)"
  else
    OS_NAME="unknown"
    OS_VERSION="unknown"
  fi
  ARCH="$(uname -m)"
  log "Detected OS: ${OS_NAME} ${OS_VERSION} arch=${ARCH}"
}

detect_asterisk() {
  if ! command -v asterisk >/dev/null 2>&1; then
    err "Asterisk binary not found in PATH. Install Asterisk before installing VoiceOTP."
  fi
  AST_VERSION="$(asterisk -V 2>/dev/null | head -1 || true)"
  log "Asterisk: ${AST_VERSION}"
  AST_BIN="$(command -v asterisk)"
}

locate_asterisk_config() {
  if [ -f "${ASTERISK_CONF}" ]; then
    ASTERISK_ETC="$(dirname "${ASTERISK_CONF}")"
  fi
  MANAGER_CONF="${ASTERISK_ETC}/manager.conf"
  EXTENSIONS_CONF="${ASTERISK_ETC}/extensions.conf"
  log "Asterisk config dir: ${ASTERISK_ETC}"
}

detect_channel_drivers() {
  PJSIP_LOADED=0
  SIP_LOADED=0
  if asterisk -rx "module show like res_pjsip" 2>/dev/null | grep -q res_pjsip; then
    PJSIP_LOADED=1
  fi
  if asterisk -rx "module show like chan_sip" 2>/dev/null | grep -q chan_sip; then
    SIP_LOADED=1
  fi
  log "Channel drivers: pjsip=${PJSIP_LOADED} chan_sip=${SIP_LOADED}"
}

verify_saydigits() {
  if asterisk -rx "core show application SayDigits" 2>/dev/null | grep -qi "SayDigits"; then
    log "SayDigits application: available"
  else
    err "SayDigits application not available. Install asterisk-sounds-core."
  fi
}

verify_sounds() {
  local lang="${1:-en}"
  local sounds_dir="${ASTERISK_VAR}/sounds"
  local ok=1
  for d in 0 1 2 3 4 5 6 7 8 9; do
    if [ ! -f "${sounds_dir}/${lang}/digits/${d}" ] && [ ! -f "${sounds_dir}/${lang}/${d}.gsm" ] && [ ! -f "${sounds_dir}/${lang}/${d}.wav" ]; then
      ok=0
      log "WARN: missing digit sound '${lang}/digits/${d}' (or .gsm/.wav)"
    fi
  done
  if [ "${ok}" -eq 1 ]; then
    log "Digit sounds for language '${lang}': present"
  else
    log "WARN: some digit sounds missing for '${lang}'; SayDigits may fall back or play tones."
  fi
}

check_ports() {
  if command -v ss >/dev/null 2>&1; then
    if ss -ltn 2>/dev/null | grep -q ':5038 '; then
      log "Port 5038 (AMI) is listening."
    else
      log "WARN: Port 5038 (AMI) not detected as listening."
    fi
  fi
}

create_service_account() {
  if ! getent group "${SERVICE_GROUP}" >/dev/null 2>&1; then
    groupadd --system "${SERVICE_GROUP}"
    log "Created group ${SERVICE_GROUP}"
  fi
  if ! getent passwd "${SERVICE_USER}" >/dev/null 2>&1; then
    useradd --system --gid "${SERVICE_GROUP}" --home "${DATA_DIR}" --shell /usr/sbin/nologin \
      --comment "VoiceOTP Gateway service account" "${SERVICE_USER}"
    log "Created user ${SERVICE_USER}"
  fi
}

create_directories() {
  install -d -m 0755 "${CONFIG_DIR}"
  install -d -m 0700 "${TLS_DIR}"
  install -d -m 0755 "${DATA_DIR}"
  install -d -m 0755 "${LOG_DIR}"
  install -d -m 0755 "${RUN_DIR}"
  chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${DATA_DIR}" "${LOG_DIR}" "${RUN_DIR}"
  log "Created configuration/data/log directories."
}

install_binary() {
  local src="${PROJECT_ROOT}/${BINARY_NAME}"
  if [ -f "${src}" ]; then
    install -m 0755 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" "${src}" "${INSTALL_BIN}"
    log "Installed prebuilt binary ${INSTALL_BIN}"
    return
  fi
  if command -v go >/dev/null 2>&1; then
    local go_src="${PROJECT_ROOT}/gateway"
    if [ -f "${go_src}/go.mod" ]; then
      (cd "${go_src}" && CGO_ENABLED=0 GOOS=linux GOARCH=$(uname -m | sed 's/x86_64/amd64/') go build -o "${INSTALL_BIN}" .)
      chown "${SERVICE_USER}:${SERVICE_GROUP}" "${INSTALL_BIN}"
      chmod 0755 "${INSTALL_BIN}"
      log "Built and installed binary from source."
      return
    fi
  fi
  err "No prebuilt binary found at ${src} and Go toolchain unavailable. Build the binary on a Linux host and copy it to ${src}."
}

install_config() {
  if [ -n "${CONFIG_SOURCE}" ]; then
    if [ ! -f "${CONFIG_SOURCE}" ]; then
      err "Config file not found: ${CONFIG_SOURCE}"
    fi
    install -m 0600 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" "${CONFIG_SOURCE}" "${CONFIG_DEST}"
    log "Installed configuration from ${CONFIG_SOURCE} to ${CONFIG_DEST}"
  elif [ ! -f "${CONFIG_DEST}" ]; then
    install -m 0600 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" "${CONFIG_EXAMPLE}" "${CONFIG_DEST}"
    log "Installed example configuration to ${CONFIG_DEST}. EDIT IT before starting the service."
  else
    log "Configuration already present at ${CONFIG_DEST}; leaving unchanged."
  fi
  chmod 0600 "${CONFIG_DEST}"
}

backup_file() {
  local f="$1"
  if [ -f "${f}" ] && [ ! -f "${f}.voiceotp.bak" ]; then
    cp -a "${f}" "${f}.voiceotp.bak"
    log "Backed up ${f} to ${f}.voiceotp.bak"
  fi
}

include_line_present() {
  local file="$1" line="$2"
  grep -Fq -- "${line}" "${file}" 2>/dev/null
}

install_ami_user() {
  local include_line="#include voiceotp_ami.conf"
  backup_file "${MANAGER_CONF}"
  if ! include_line_present "${MANAGER_CONF}" "${include_line}"; then
    printf '\n; VoiceOTP Gateway AMI user\n%s\n' "${include_line}" >> "${MANAGER_CONF}"
    log "Added AMI include to ${MANAGER_CONF}"
  else
    log "AMI include already present in ${MANAGER_CONF}"
  fi
  if [ ! -f "${AMI_FILE}" ]; then
    install -m 0600 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" "${AMI_EXAMPLE}" "${AMI_FILE}"
    log "Installed AMI user file ${AMI_FILE}. EDIT the secret before reloading Asterisk."
  else
    log "AMI user file already present at ${AMI_FILE}; leaving unchanged."
  fi
  chmod 0600 "${AMI_FILE}"
}

install_dialplan() {
  local include_line="#include voiceotp-playback.conf"
  backup_file "${EXTENSIONS_CONF}"
  if ! include_line_present "${EXTENSIONS_CONF}" "${include_line}"; then
    printf '\n; VoiceOTP Gateway dialplan context\n%s\n' "${include_line}" >> "${EXTENSIONS_CONF}"
    log "Added dialplan include to ${EXTENSIONS_CONF}"
  else
    log "Dialplan include already present in ${EXTENSIONS_CONF}"
  fi
  if [ ! -f "${DIALPLAN_FILE}" ]; then
    install -m 0644 -o "${SERVICE_USER}" -g "${SERVICE_GROUP}" "${PROJECT_ROOT}/issabel/asterisk/voiceotp-playback.conf" "${DIALPLAN_FILE}"
    log "Installed dialplan context ${DIALPLAN_FILE}"
  else
    log "Dialplan context already present at ${DIALPLAN_FILE}; leaving unchanged."
  fi
}

install_systemd() {
  install -m 0644 "${UNIT_SOURCE}" "${SYSTEMD_UNIT}"
  systemctl daemon-reload
  systemctl enable voiceotp.service
  log "Installed and enabled systemd unit ${SYSTEMD_UNIT}"
}

start_service() {
  if systemctl is-active --quiet voiceotp.service; then
    systemctl restart voiceotp.service
    log "Restarted voiceotp.service"
  else
    systemctl start voiceotp.service
    log "Started voiceotp.service"
  fi
  sleep 1
  if systemctl is-active --quiet voiceotp.service; then
    log "voiceotp.service is running."
  else
    log "WARN: voiceotp.service is not running. Check journalctl -u voiceotp.service."
  fi
}

main() {
  require_root
  detect_os
  detect_asterisk
  locate_asterisk_config
  detect_channel_drivers
  verify_saydigits
  verify_sounds en
  check_ports
  create_service_account
  create_directories
  install_binary
  install_config
  install_ami_user
  install_dialplan
  install_systemd
  start_service
  log "Installation complete. Next steps:"
  log "  1. Edit ${CONFIG_DEST} with your API credentials, AMI secret, and trunk dial format."
  log "  2. Edit ${AMI_FILE} with a high-entropy AMI secret."
  log "  3. Reload Asterisk: asterisk -rx 'manager reload' && asterisk -rx 'dialplan reload'"
  log "  4. Restart VoiceOTP: systemctl restart voiceotp"
}

main "$@"
