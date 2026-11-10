#!/usr/bin/env bash
#
# VoiceOTP Gateway - test-call script
#
# Sends a test OTP through the gateway using curl. Intended for manual
# verification with a real telephone trunk. Does NOT place paid calls in CI.
#
# Usage: ./issabel/scripts/test-call.sh <number> <otp>
set -euo pipefail

CONFIG="${CONFIG:-/etc/voiceotp/config.json}"
GATEWAY_URL="${GATEWAY_URL:-https://127.0.0.1:8443}"
API_USERNAME="${API_USERNAME:-voiceotp_api}"

if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <number> <otp>"
  echo "Example: $0 +989121234567 004829"
  exit 1
fi

NUMBER="$1"
OTP="$2"

read -s -p "API password for ${API_USERNAME}: " API_PASSWORD
echo

BODY=$(printf '{"username":"%s","password":"%s","otp":"%s","number":"%s"}' \
  "${API_USERNAME}" "${API_PASSWORD}" "${OTP}" "${NUMBER}")

HTTP_CODE=$(curl -s -o /tmp/voiceotp-test-response.txt -w "%{http_code}" \
  --cacert /etc/voiceotp/tls/server.crt \
  --header "Content-Type: application/json" \
  --data "${BODY}" \
  "${GATEWAY_URL}/api/v1/voice-otp" || true)

echo "HTTP ${HTTP_CODE}"
cat /tmp/voiceotp-test-response.txt
echo
rm -f /tmp/voiceotp-test-response.txt
