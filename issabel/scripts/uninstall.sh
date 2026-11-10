#!/usr/bin/env bash
#
# VoiceOTP Gateway - uninstall script
#
# Removes only files and configuration entries installed by this project.
# Does NOT delete existing SIP trunks, extensions, routes, recordings, or
# unrelated PBX configuration.
#
# Usage: sudo ./issabel/scripts/uninstall.sh
set -euo pipefail

ASTERISK_ETC="${ASTERISK_ETC:-/etc/asterisk}"
CONFIG_DIR="/etc/voiceotp"
TLS_DIR="${CONFIG_DIR}/tls"
DATA_DIR="/var/lib/voiceotp"
LOG_DIR="/var/log/voiceotp"
RUN_DIR="/var/run/voiceotp"
SERVICE_USER="voiceotp"
SERVICE_GROUP="voiceotp"
SYSTEMD_UNIT="/etc/systemd/system/voiceotp.service"
INSTALL_BIN="/usr/local/bin/voiceotp-gateway"
MANAGER_CONF="${ASTERISK_ETC}/manager.conf"
EXTENSIONS_CONF="${ASTERISK_ETC}/extensions.conf"
DIALPLAN_FILE="${ASTERISK_ETC}/voiceotp-playback.conf"
AMI_FILE="${ASTERISK_ETC}/voiceotp_ami.conf"

log() { printf '[uninstall] %s\n' "$*"; }
err() { printf '[uninstall][ERROR] %s\n' "$*" >&2; exit 1; }

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    err "This script must be run as root (use sudo)."
  fi
}

remove_include_line() {
  local file="$1" line="$2"
  if [ -f "${file}" ] && grep -Fq -- "${line}" "${file}" 2>/dev/null; then
    grep -v -F -- "${line}" "${file}" > "${file}.tmp" && mv "${file}.tmp" "${file}"
    log "Removed include line from ${file}"
  fi
}

main() {
  require_root

  log "Stopping and disabling voiceotp.service..."
  if systemctl list-unit-files 2>/dev/null | grep -q 'voiceotp.service'; then
    systemctl disable --now voiceotp.service 2>/dev/null || true
    rm -f /etc/systemd/system/voiceotp.service
    systemctl daemon-reload
    log "Removed systemd unit."
  fi

  log "Removing binary ${INSTALL_BIN}..."
  rm -f "${INSTALL_BIN}"

  log "Removing VoiceOTP configuration and data directories..."
  rm -rf "${CONFIG_DIR}" "${DATA_DIR}" "${LOG_DIR}" "${RUN_DIR}"

  log "Removing Asterisk dialplan include and context file..."
  remove_include_line "${EXTENSIONS_CONF}" "#include voiceotp-playback.conf"
  rm -f "${DIALPLAN_FILE}"

  log "Removing AMI user include and file..."
  remove_include_line "${MANAGER_CONF}" "#include voiceotp_ami.conf"
  rm -f "${AMI_FILE}"

  log "Removing service account '${SERVICE_USER}'..."
  if getent passwd "${SERVICE_USER}" >/dev/null 2>&1; then
    userdel "${SERVICE_USER}" 2>/dev/null || true
  fi
  if getent group "${SERVICE_GROUP}" >/dev/null 2>&1; then
    groupdel "${SERVICE_GROUP}" 2>/dev/null || true
  fi

  log "Uninstall complete. PBX trunk/extension configuration was not touched."
  log "Reload Asterisk to apply: asterisk -rx 'manager reload' && asterisk -rx 'dialplan reload'"
}

main "$@"
