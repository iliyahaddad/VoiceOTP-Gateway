# API Reference

## Endpoint: POST /api/v1/voice-otp

Content-Type: `application/json`. HTTPS only.

### Request

```json
{
  "username": "voiceotp_api",
  "password": "REPLACE_WITH_SECRET",
  "otp": "004829",
  "number": "+989121234567"
}
```

All four fields are required. Credentials must never be sent in the URL or
query string.

### Validation

- `otp`: string of digits, length configurable (default 4–8). Leading zeros
  preserved. Never cast to an integer.
- `number`: E.164-style international number (`^\+[0-9]{7,15}$`).

### Response: accepted (queued)

HTTP 202

```json
{
  "ok": true,
  "status": "queued",
  "request_id": "req-34e7bff73b2da57a"
}
```

HTTP 202 means Asterisk accepted the request for asynchronous processing. It
does NOT mean the recipient answered or heard the OTP.

### Error responses

| Code | Meaning |
|------|---------|
| 400  | Validation failure (missing field, bad OTP, bad number, malformed JSON) |
| 401  | Authentication failure |
| 403  | Source IP not allowlisted |
| 429  | Rate limit exceeded |
| 502  | AMI unavailable or call rejected |
| 503  | Too many concurrent calls |

### Endpoint: GET /healthz

HTTP 200 `{"status":"ok"}`. No credentials or internal details exposed.

## Idempotency

Optional `Idempotency-Key` header. Duplicate keys within the TTL are rejected
with HTTP 409.

## Example (cURL)

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

Do not use `-k` or disable TLS verification in production.

## WordPress example

```php
$result = \VoiceOTP\Connector\Client::send( '+989121234567', '004829' );
if ( $result['success'] ) {
    // HTTP 202: call queued for asynchronous processing.
}
```

## PHP example

```php
$ch = curl_init('https://PBX-HOST:8443/api/v1/voice-otp');
curl_setopt($ch, CURLOPT_POST, true);
curl_setopt($ch, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode([
    'username' => 'voiceotp_api',
    'password' => 'YOUR_API_PASSWORD',
    'otp' => '004829',
    'number' => '+989121234567',
]));
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
$response = curl_exec($ch);
curl_close($ch);
```
