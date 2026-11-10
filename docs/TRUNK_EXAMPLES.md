# Trunk Configuration Examples

The gateway never derives the channel type or trunk from the API request.
The administrator configures `channel_driver`, `trunk_name`, and `dial_format`
in `config.json`. The `dial_format` uses `%s` as a placeholder for the
normalized E.164 number.

## PJSIP (preferred)

Example trunk endpoint `voiceotp_trunk`:

```json
"asterisk": {
  "channel_driver": "pjsip",
  "trunk_name": "voiceotp_trunk",
  "dial_format": "PJSIP/voiceotp_trunk/%s",
  "caller_id": "VoiceOTP <1000>"
}
```

If your dialplan uses an AOR, you may need `PJSIP/<aor>/%s` or a custom
dialplan subroutine. Detect the correct syntax at install time with
`asterisk -rx "pjsip show endpoints"`.

## chan_sip (legacy)

Example trunk `voiceotp_trunk`:

```json
"asterisk": {
  "channel_driver": "sip",
  "trunk_name": "voiceotp_trunk",
  "dial_format": "SIP/voiceotp_trunk/%s",
  "caller_id": "VoiceOTP <1000>"
}
```

## Important

- Never assume `PJSIP/mytrunk/number` is valid for every trunk.
- Never concatenate caller-supplied input into the AMI Channel value.
- Do not overwrite existing trunk configuration; do not auto-create a trunk.
