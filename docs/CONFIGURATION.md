# Configuration Reference

## File: /etc/voiceotp/config.json

Permissions: 0600, owned by `voiceotp:voiceotp`.

Example (`issabel/config/config.example.json`):

```json
{
  "server": {
    "listen": "127.0.0.1:8443",
    "tls_enabled": true,
    "tls_cert": "/etc/voiceotp/tls/server.crt",
    "tls_key": "/etc/voiceotp/tls/server.key",
    "read_timeout_seconds": 5,
    "write_timeout_seconds": 10
  },
  "api": {
    "username": "voiceotp_api",
    "password": "REPLACE_WITH_HIGH_ENTROPY_SECRET",
    "otp_min_length": 4,
    "otp_max_length": 8,
    "max_request_body_bytes": 4096
  },
  "ami": {
    "host": "127.0.0.1",
    "port": 5038,
    "username": "voiceotp_ami",
    "secret": "REPLACE_WITH_HIGH_ENTROPY_AMI_SECRET",
    "timeout_seconds": 3
  },
  "asterisk": {
    "channel_driver": "pjsip",
    "trunk_name": "YOUR_TRUNK",
    "dial_format": "PJSIP/YOUR_TRUNK/%s",
    "context": "voiceotp-playback",
    "caller_id": "VoiceOTP <1000>",
    "language": "en",
    "call_timeout_ms": 30000
  },
  "security": {
    "allowed_client_cidrs": ["127.0.0.1/32"],
    "max_requests_per_minute": 10,
    "max_concurrent_calls": 5
  }
}
```

### server

- `listen`: bind address. Default `127.0.0.1:8443`. For remote WordPress,
  bind to a private network address or secured public interface with TLS.
- `tls_enabled`: enable HTTPS. For remote access this must be `true`.
- `tls_cert`, `tls_key`: paths to certificate and key.
- `read_timeout_seconds`, `write_timeout_seconds`: HTTP timeouts.

### api

- `username`, `password`: API credentials. The password must be high-entropy,
  never `CHANGE_THIS`.
- `otp_min_length` / `otp_max_length`: accepted OTP digit length.
- `max_request_body_bytes`: maximum request body size.

### ami

- `host`, `port`: AMI endpoint (default 127.0.0.1:5038).
- `username`, `secret`: dedicated AMI credentials. Independent from API
  password.
- `timeout_seconds`: AMI connect/read/write deadline.

### asterisk

- `channel_driver`: `pjsip` (preferred) or `sip`.
- `trunk_name`: your outbound trunk endpoint/AOR name.
- `dial_format`: dial string format with `%s` for the number, e.g.
  `PJSIP/YOUR_TRUNK/%s`. Never derived from the API request.
- `context`: `voiceotp-playback`.
- `caller_id`: caller ID for outbound calls.
- `language`: channel language for SayDigits sound files (`en`, `fa`, ...).
- `call_timeout_ms`: originate timeout in milliseconds.

### security

- `allowed_client_cidrs`: CIDR list of permitted API source IPs.
- `max_requests_per_minute`: rate limit per client.
- `max_concurrent_calls`: max simultaneous outbound calls.

## Validation

All settings are validated at startup. The service refuses to start with
invalid or incomplete mandatory settings (example credentials rejected,
missing trunk, invalid CIDR, etc.).

## AMI user

`issabel/config/voiceotp_ami.conf.example` defines a dedicated AMI user
permitted only `originate`/`command` from 127.0.0.1. Install via the custom
include mechanism and set a high-entropy secret.

## Language / sounds

`SayDigits()` uses the channel language's sound files. Verify digit sounds
exist for the selected language (English and Persian subject to installed
packages). The gateway provides a diagnostic (verify.sh) but does not claim
Persian works unless the sounds are installed.
