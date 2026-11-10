#!/usr/bin/env bash
#
# VoiceOTP Gateway - installation verification script
#
# Checks that the gateway binary, configuration, systemd unit, Asterisk AMI
# user, dialplan context, and sounds are present and consistent.
#
# Usage: sudo ./issabel/scripts/verify.sh
set -euo pipefail

ASTERISK_ETC="${ASTERISK_ETC:-/etc/asterisk}"
CONFIG_DIR="/etc/voiceotp"
INSTALL_BIN="/usr/local/bin/voiceotp-gateway"
SYSTEMD_UNIT="/etc/systemd/system/voiceotp.service"
MANAGER_CONF="${ASTERISK_ETC}/manager.conf"
EXTENSIONS_CONF="${ASTERISK_ETC}/extensions.conf"
DIALPLAN_FILE="${ASTERISK_ETC}/voiceotp-playback.conf"
AMI_FILE="${ASTERISK_ETC}/voiceotp_ami.conf"
FAILURES=0

ok() { printf '[verify][OK]   %s\n' "$*"; }
warn() { printf '[verify][WARN] %s\n' "$*"; }
fail() { printf '[verify][FAIL] %s\n' "$*"; FAILURES=$((FAILURES+1)); }

check_file() { [ -f "$1" ] && ok "$1 present" || fail "$1 missing"; }
check_exec() { [ -x "$1" ] && ok "$1 executable" || fail "$1 not executable"; }
check_include() {
  local file="$1" line="$2"
  if [ -f "${file}" ] && grep -Fq -- "${line}" "${file}" 2>/dev/null; then
    ok "${file} includes ${line}"
  else
    fail "${file} missing include ${line}"
  fi
}

echo "=== VoiceOTP Gateway verification ==="

check_exec "${INSTALL_BIN}"
check_file "${SYSTEMD_UNIT}"
if systemctl is-active --quiet voiceotp.service 2>/dev/null; then
  ok "voiceotp.service active"
else
  warn "voiceotp.service not active (start with: systemctl start voiceotp)"
fi
if systemctl is-enabled --quiet voiceotp.service 2>/dev/null; then
  ok "voiceotp.service enabled"
else
  warn "voiceotp.service not enabled"
fi

[ -d /etc/voiceotp ] && ok "/etc/voiceotp present" || fail "/etc/voiceotp missing"
[ -f /etc/voiceotp/config.json ] && ok "config.json present" || fail "config.json missing"

check_include "${MANAGER_CONF}" "#include voiceotp_ami.conf"
check_file "${AMI_FILE}"
check_include "${EXTENSIONS_CONF}" "#include voiceotp-playback.conf"
check_file "${DIALPLAN_FILE}"

if command -v asterisk >/dev/null 2>&1; then
  if asterisk -rx "manager show user voiceotp_ami" 2>/dev/null | grep -q voiceotp_ami; then
    ok "AMI user voiceotp_ami registered"
  else
    fail "AMI user voiceotp_ami not registered (run: manager reload)"
  fi
  if asterisk -rx "dialplan show voiceotp-playback" 2>/dev/null | grep -qi voiceotp-playback; then
    ok "Dialplan context voiceotp-playback registered"
  else
    fail "Dialplan context voiceotp-playback not registered (run: dialplan reload)"
  fi
  if asterisk -rx "core show application SayDigits" 2>/dev/null | grep -qi SayDigits; then
    ok "SayDigits application available"
  else
    fail "SayDigits application not available"
  fi
else
  warn "asterisk CLI not available"
fi

echo
if [ "${FAILURES}" -eq 0 ]; then
  echo "Verification: ALL CHECKS PASSED"
  exit 0
else
  echo "Verification: ${FAILURES} CHECK(S) FAILED"
  exit 1
fi
