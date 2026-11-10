# Troubleshooting

## AMI authentication failure

- Check `/etc/asterisk/voiceotp_ami.conf` secret matches `config.json` `ami.secret`.
- Run `asterisk -rx "manager show user voiceotp_ami"`.
- Reload: `asterisk -rx "manager reload"`.
- Logs: `journalctl -u voiceotp` (category: `ami_error` / `auth_failed`).

## Connection refused on port 5038

- Confirm AMI is enabled: `asterisk -rx "manager show settings"`.
- Confirm `bindaddr = 127.0.0.1` in `manager.conf` and port 5038.
- Confirm the gateway config `ami.host`/`ami.port`.

## SIP trunk not found

- Verify `asterisk.trunk_name` and `dial_format` match your trunk.
- PJSIP: `asterisk -rx "pjsip show endpoints"`.
- chan_sip: `asterisk -rx "sip show peers"`.

## PJSIP endpoint resolution failure

- Confirm the endpoint/AOR in `dial_format` exists and is allowed to dial out.
- Check Asterisk logs: `tail -f /var/log/asterisk/full`.

## chan_sip unavailable

- The installed Asterisk may not load `chan_sip`. Set `channel_driver: pjsip`
  or load `chan_sip.so` and restart Asterisk.

## Call rejected

- `502` from the gateway means AMI rejected the Originate. Check the AMI user
  permissions (`write = command,originate`) and trunk validity.

## No answer / no audio

- 202 only means the call was queued. Check the recipient answered.
- Confirm `SayDigits()` plays: `asterisk -rx "core show application SayDigits"`.
- Confirm digit sounds exist: `/var/lib/asterisk/sounds/<lang>/digits/`.

## Missing digit sound files / incorrect language

- Set `asterisk.language` to an installed sound language.
- Verify sounds for all ten digits in that language.
- For Persian (`fa`), install a compatible sound package; the gateway does not
  claim `fa` works without the sounds.

## TLS certificate problems

- Do not disable TLS verification. Add the CA to trust or use a valid cert.
- WordPress: keep `voiceotp_verify_ssl` enabled.

## WordPress HTTP timeout

- Increase `voiceotp_timeout` in Settings → VoiceOTP.
- Confirm the gateway URL is reachable from the WordPress server.

## Gateway service

```bash
systemctl status voiceotp
journalctl -u voiceotp -n 50
```

Logs never contain OTPs or passwords.
