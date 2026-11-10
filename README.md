# VoiceOTP Gateway

Self-hosted, voice-based OTP delivery gateway for Issabel / Asterisk.

Receive an authenticated HTTPS request, call a recipient through a configured
SIP/PJSIP trunk, pronounce the OTP digit-by-digit with Asterisk's native
`SayDigits()`, and hang up.

## What it does

- Authenticates API callers (constant-time username/password over HTTPS).
- Validates OTP (digits only, configurable length; leading zeros preserved).
- Validates telephone numbers (E.164-style; no SIP/channel injection).
- Originates an outbound call via Asterisk AMI (localhost only).
- Plays the OTP digits via the isolated `[voiceotp-playback]` dialplan context
  using `SayDigits()`, then hangs up.
- Returns HTTP 202 with a unique `request_id` (call queued asynchronously).

## What it does NOT do

No TTS, no SMS, no email, no database, no Redis, no Docker, no Node.js, no
Composer, no external API, no OTP generation/verification, no greeting/menu,
no DTMF, no call recording.

## Architecture

```
WordPress (or caller)
      | HTTPS POST /api/v1/voice-otp
      v
VoiceOTP Gateway (Go, systemd, 127.0.0.1:8443)
      | AMI (TCP 127.0.0.1:5038, dedicated user)
      v
Asterisk / Issabel
      | Originate via configured SIP/PJSIP trunk
      v
Recipient answers -> [voiceotp-playback] -> SayDigits(OTP) -> Hangup
```

## Repository structure

```
issabel/          Gateway: Go API, AMI client, dialplan, scripts, systemd
wordpress/        VoiceOTP Connector WordPress plugin
docs/             Architecture, API, configuration, installation, troubleshooting
tests/            Test documentation
.github/          CI workflow
```

## Quick start (Issabel server)

```bash
sudo ./issabel/scripts/install.sh /etc/voiceotp/config.json
```

See `docs/INSTALL_ISSABEL.md`. The installer detects Asterisk, channel drivers,
verifies `SayDigits()` and sounds, installs the binary, AMI user, dialplan
context, and systemd service. It never overwrites existing PBX configuration.

Edit `/etc/voiceotp/config.json` with your API credentials, AMI secret, and
trunk dial format before starting.

## Trunk configuration

Set `channel_driver`, `trunk_name`, and `dial_format` in `config.json`.

- PJSIP: `"dial_format": "PJSIP/<endpoint>/ %s"` (no space).
- chan_sip: `"dial_format": "SIP/<trunk>/ %s"`.

See `docs/TRUNK_EXAMPLES.md`. Detect your actual syntax at install time;
never assume `PJSIP/mytrunk/number`.

## API usage

```bash
curl --request POST \
  --url "https://PBX-HOST:8443/api/v1/voice-otp" \
  --header "Content-Type: application/json" \
  --data '{
    "username": "voiceotp_api",
    "password": "YOUR_API_PASSWORD",
    "otp": "004829",
    "number": "+989121234567"
  }'
```

Response (HTTP 202):

```json
{ "ok": true, "status": "queued", "request_id": "req-..." }
```

202 means the call was queued for asynchronous processing; it does not confirm
delivery. See `docs/API.md`.

## WordPress connector

Install `wordpress/voiceotp-connector/` as a plugin, configure **Settings →
VoiceOTP**, then use:

```php
$result = \VoiceOTP\Connector\Client::send( '+989121234567', '004829' );
```

An administrator-only test page is at **Settings → VoiceOTP → Test Connection**.
See `docs/WORDPRESS.md`.

## Running tests

```bash
cd issabel/gateway
go test ./... -timeout 60s
go vet ./...
go build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/voiceotp-gateway .
```

PHP: `php -l` on all plugin files. Shell: `bash -n` on all scripts.

CI: `.github/workflows/ci.yml`.

## Security

See `SECURITY.md`. Highlights: HTTPS-only, constant-time comparison, independent
API/AMI credentials, IP allowlisting, rate limiting, no secrets in logs,
restricted config permissions, dedicated service user, AMI never public.

## Known limitations

- Tested on Go 1.23; requires a Linux Issabel host for the gateway binary.
- Asterisk integration tests require a live PBX with a telephone trunk; CI
  uses a fake AMI server.
- PHP/Asterisk tooling is not available on this development workstation; shell
  and PHP syntax checks run in CI.
- Persian (`fa`) pronunciation works only if the corresponding Asterisk digit
  sound files are installed.

## Supported versions

- Go 1.23+ (gateway).
- Asterisk via AMI (Issabel-standard).
- WordPress 6.x (connector).

## License

See `LICENSE`.
