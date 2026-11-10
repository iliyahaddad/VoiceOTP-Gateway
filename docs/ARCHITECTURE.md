# Architecture

## Overview

VoiceOTP Gateway does exactly one job: receive an authenticated HTTPS POST
containing a username, password, OTP, and telephone number; validate the
request; instruct Asterisk (via AMI) to call the recipient through a configured
trunk; pronounce the OTP digit-by-digit using Asterisk's native `SayDigits()`
application; and hang up.

It does not generate, verify, store, or manage OTP codes. That belongs to the
calling application (e.g. WordPress).

## Components

```
voiceotp-gateway/
├── issabel/gateway/          # Go API service + AMI client (+ tests)
├── issabel/asterisk/         # Dialplan context voiceotp-playback.conf
├── issabel/config/           # Configuration templates
├── issabel/scripts/          # install / uninstall / verify / test-call
├── issabel/systemd/          # voiceotp.service
├── wordpress/voiceotp-connector/   # WordPress plugin
├── docs/
├── tests/
├── .github/workflows/ci.yml
└── LICENSE, README.md, SECURITY.md, CONTRIBUTING.md, CHANGELOG.md
```

## Gateway (Issabel server)

- **Language:** Go, standard library only.
- **Binary:** statically linked Linux x86-64 (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`).
- **Service:** systemd, runs as dedicated unprivileged `voiceotp` user.
- **API:** `POST /api/v1/voice-otp` (JSON, HTTPS), `GET /healthz`.
- **AMI:** native client over TCP to `127.0.0.1:5038` (localhost only).
- **Dialplan:** isolated context `[voiceotp-playback]`, not publicly dialable.

## Data flow

1. WordPress (or caller) POSTs JSON to `https://pbx:8443/api/v1/voice-otp`.
2. Gateway authenticates (constant-time), validates OTP (`^[0-9]{4,8}$`) and
   number (E.164), checks IP allowlist and rate limit.
3. Gateway connects to AMI, logs in with a dedicated AMI user, sends
   `Originate` with `Variable: VOICEOTP_CODE=<otp>` and a unique `ActionID`.
4. Asterisk calls the recipient via the configured trunk.
5. On answer, the `[voiceotp-playback]` context answers, waits, plays the
   digits with `SayDigits(${VOICEOTP_CODE})`, and hangs up.
6. Gateway returns `202 Accepted` with a `request_id`. This means the call was
   queued for asynchronous processing — not that delivery is confirmed.

## Trust boundaries

- AMI port 5038 must never be exposed publicly; bind to 127.0.0.1.
- API credentials and AMI credentials are independent and stored outside Git.
- TLS is required for remote access; loopback default.

## What it does NOT do

No TTS, no SMS, no email, no database, no Redis, no Docker, no Node.js, no
Composer, no external API, no greeting/menu/DTMF/repeat loop, no call
recording, no OTP generation or verification.
