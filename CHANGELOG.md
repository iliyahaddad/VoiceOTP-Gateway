# Changelog

## [1.0.0] - 2026-11-10

### Added
- VoiceOTP Gateway Go service: REST API (`POST /api/v1/voice-otp`, `GET /healthz`).
- Native Asterisk AMI client (no external library): greeting handling, Login, Originate with ActionID and Variable headers, Logoff.
- Dedicated Asterisk dialplan context `[voiceotp-playback]` using `SayDigits()`.
- Go configuration loader with validation (JSON config at /etc/voiceotp/config.json).
- IP allowlisting, rate limiting, idempotency keys, bounded concurrent calls.
- Structured logging without OTP/password exposure.
- systemd unit for dedicated unprivileged service user.
- Issabel installation/uninstall/verify/test-call scripts.
- PJSIP and chan_sip trunk support documentation and examples.
- WordPress Connector plugin: Settings API page, wp_remote_post client, developer interface `Client::send()`, admin-only test feature.
- GitHub Actions CI for Go build/vet/test and PHP lint.
- Documentation: architecture, API, installation, configuration, troubleshooting, security.
