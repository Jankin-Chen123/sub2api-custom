# Cangyuan admin image credential test

This endpoint performs one real, billable upstream generation to verify an
`image_only` account. It does not create a user image job and does not charge a
user balance.

## Request

```http
POST /api/v1/admin/accounts/{id}/test-image
Content-Type: application/json
```

```json
{
  "confirm": true,
  "model": "gpt-image-2-1k",
  "prompt": "A simple blue circle on a white background"
}
```

Rules:

- `confirm` must be `true`; this is the second confirmation for upstream cost.
- The account must have `extra.account_purpose = "image_only"`.
- An omitted `model` defaults to `gpt-image-2-1k`.
- Allowed models are `gpt-image-2-1k`, `gpt-image-2-2k`, and
  `gpt-image-2-4k`.
- `size`, references, and `mask` are not accepted by this test endpoint.
  Use the Images API or workbench for those cases.

## Response

Successful response:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "model": "gpt-image-2-1k",
    "status": "completed",
    "completed": true,
    "duration_ms": 12345
  }
}
```

The response never contains the provider key, provider task ID, signed output
URL, or raw upstream response body.

| Status | Meaning |
| ---: | --- |
| 200 | Generation completed |
| 400 | Missing confirmation, non-image account, or unsupported model |
| 404 | Account does not exist |
| 502 | Cangyuan authentication, network, timeout, or generation failure |

The frontend exposes this operation in the admin account test dialog. For an
image-only account it shows fixed 1K/2K/4K Cangyuan models and requires an
explicit cost-confirmation checkbox before sending the request.
