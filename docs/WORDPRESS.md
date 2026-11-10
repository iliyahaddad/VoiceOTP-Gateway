# WordPress Installation Guide

## Install

1. Copy `wordpress/voiceotp-connector/` to `wp-content/plugins/voiceotp-connector/`.
2. Activate the plugin in the WordPress admin.
3. Go to **Settings → VoiceOTP** and configure:
   - Gateway HTTPS URL (e.g. `https://pbx.example.com:8443`)
   - API username
   - API password
   - Connection timeout
   - Enable connector
4. Save changes.

## Developer integration

```php
$result = \VoiceOTP\Connector\Client::send( '+989121234567', '004829' );
if ( $result['success'] ) {
    // HTTP 202: call queued for asynchronous processing.
} else {
    // $result['error'] is a WP_Error.
}
```

A helper function `voiceotp_send_otp( $number, $otp )` is also available.

## Administrator test

**Settings → VoiceOTP → Test Connection** (manage_options capability, nonce
protected). Enter a destination and test OTP, then send. This places a real
call.

## Security

- Uses `wp_remote_post()` with TLS verification enabled by default.
- Never sends credentials through frontend JavaScript.
- Password is never shown in readable form; an empty password field keeps the
  saved value.
- Nonce-verified admin actions; `manage_options` capability required.
