# VoiceOTP Gateway Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

Please report security vulnerabilities privately to the project maintainers
before disclosing them publicly. Do not open a public issue for a security
vulnerability.

## Scope

In scope:
- VoiceOTP Gateway Go service (authentication, rate limiting, AMI client).
- WordPress Connector plugin (settings, nonce handling, HTTP client).

Out of scope:
- The underlying Issabel/Asterisk PBX and its configuration.
- The security of the telephone network or trunk.

## Security Measures

- All API credentials transmitted only over TLS (HTTPS).
- Constant-time credential comparison.
- Independent API and AMI credentials (never reused).
- IP allowlisting for API clients.
- Configurable rate limiting and bounded concurrent calls.
- Strict OTP validation (digits only, configurable length).
- Strict telephone number validation (E.164-style, no SIP/channel injection).
- No OTP or password in logs or error responses.
- Restricted configuration file permissions (0600).
- Dedicated unprivileged Linux service user.
- AMI bound to localhost; never exposed publicly.
- Bounded memory for rate-limit and idempotency tracking.
- Safe shutdown and request cancellation.
