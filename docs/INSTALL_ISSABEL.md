# Issabel Installation Guide

## Prerequisites

- Issabel PBX with Asterisk installed.
- Root access.
- Port 5038 (AMI) reachable locally; not exposed publicly.
- TLS certificate/key for HTTPS (self-signed acceptable for private networks).

## Steps

1. Copy the project to the Issabel server.
2. Review `issabel/config/config.example.json` and create
   `/etc/voiceotp/config.json` with your credentials and trunk dial format.
3. Run the installer:

   ```bash
   sudo ./issabel/scripts/install.sh /etc/voiceotp/config.json
   ```

   The installer:
   - Detects OS, Asterisk version, channel drivers (PJSIP/chan_sip).
   - Verifies `SayDigits()` and digit sound availability.
   - Creates the `voiceotp` service user and directories.
   - Installs the static Go binary.
   - Installs the dedicated AMI user via a custom include in `manager.conf`.
   - Installs the `[voiceotp-playback]` dialplan context via a custom include
     in `extensions.conf` (no generated files are overwritten).
   - Installs and enables the `voiceotp` systemd service.

4. Edit the AMI secret in `/etc/asterisk/voiceotp_ami.conf` and reload:

   ```bash
   asterisk -rx 'manager reload'
   asterisk -rx 'dialplan reload'
   ```

5. Restart the gateway:

   ```bash
   systemctl restart voiceotp
   ```

6. Verify:

   ```bash
   sudo ./issabel/scripts/verify.sh
   ```

## Trunk configuration

Set `asterisk.channel_driver`, `asterisk.trunk_name`, and
`asterisk.dial_format` in `config.json` to match your outbound trunk.

- PJSIP: `PJSIP/<endpoint-or-aor>/%s` or `PJSIP/<trunk>/%s` depending on your
  dialplan.
- chan_sip: `SIP/<trunk>/%s`.

See `docs/TRUNK_EXAMPLES.md`. Never hardcode `PJSIP/mytrunk/number`; detect
your actual channel-driver syntax at install time.

## Service management

```bash
systemctl start voiceotp
systemctl stop voiceotp
systemctl restart voiceotp
systemctl status voiceotp
journalctl -u voiceotp
```

## Rollback / uninstall

```bash
sudo ./issabel/scripts/uninstall.sh
```

Removes only project-installed files and include lines. Existing PBX
configuration is preserved. Reload Asterisk afterward.
