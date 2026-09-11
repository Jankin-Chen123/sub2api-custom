---
name: cangyuan-api-gateway
description: Integrate 沧元算力 public APIs. Use for model discovery, OpenAI-compatible chat, image and audio calls, async video tasks, token usage checks, and model-specific media rules.
---

# 沧元算力接入

## Configuration

Read these variables from the project root `.env`. Never put an API key in browser code, source control, logs, or responses.

```dotenv
CANGYUAN_BASE_URL=http://direct-api.cangyuansuanli.cn
CANGYUAN_API_KEY=sk-your-api-key
```

If the user was issued another gateway address, use that address instead. Remove a trailing `/v1` from `CANGYUAN_BASE_URL`; endpoints below include `/v1` themselves.

Before writing the key, ensure `.env` is listed in `.gitignore`. If `.env` exists, append only the missing `CANGYUAN_*` variables and preserve all existing values.

## Operating Rules

1. Load `CANGYUAN_BASE_URL` and `CANGYUAN_API_KEY` from `.env` before making a request.
2. First call `GET {BASE_URL}/v1/models` with `Authorization: Bearer {API_KEY}`. This is the live source of models visible to this exact token.
3. Do not invent a model name, price, or parameter. Public contracts and model field subsets are documented on this site under `/docs`.
4. Use the least invasive read-only request first. Do not submit paid generation jobs until the user explicitly asks to generate.
5. Treat result URLs as opaque. Use the returned URL directly; do not rewrite its host or extract upstream details. Some result URLs expire — download promptly.
6. Never retry an accepted async media task by resubmitting it merely because the client timed out. Poll its existing task ID first.
7. Do not generate images or videos through Chat, Responses, or Messages. Use the image / video entrypoints below.

## Verify The Token And Discover Models

```bash
curl -sS "$CANGYUAN_BASE_URL/v1/models" \
  -H "Authorization: Bearer $CANGYUAN_API_KEY"
```

The response is OpenAI-style:

```json
{
  "object": "list",
  "data": [{"id": "public-model-name", "object": "model"}]
}
```

Only recommend IDs returned by this call. A model omitted from this response is not available to the current token or token group.

`GET /v1/models/{id}` returns one model. `GET /api/pricing` is the live price and capability-tag view — read it, do not invent prices.

## Calling Entrypoints

| Need | Endpoint | Behavior |
|---|---|---|
| Chat / text / multimodal | `POST /v1/chat/completions` | OpenAI-compatible; `stream: true` for SSE. Not for image or video generation. |
| Responses | `POST /v1/responses` | OpenAI Responses; compact at `/v1/responses/compact`. |
| Claude Messages | `POST /v1/messages` | Anthropic-compatible. |
| Gemini | `/v1beta/models/{model}:generateContent` | Gemini native. List shape: `GET /v1beta/models`. |
| Image generation / edits | `POST /v1/images/generations`, `/v1/images/edits` | New calls: `"async": true`, then poll the same path `GET …/{task_id}`. On `completed`, return `data[0].url`. `/v1/images/variations` is unimplemented. |
| Video generation | `POST /v1/videos` | Async; poll `GET /v1/videos/{task_id}`. On `completed`, return `video_url` or `data[0].url`. Do not GET `/content` to pull the file through this API. Remix only when the model page lists it. |
| Video assets | `POST /v1/videos/assets`, `GET /v1/videos/assets/{asset_id}` | Only when the model page lists them. Query must include `model`. Wait until `Active` before citing `asset://{asset_id}`. |
| Audio generation | `POST /v1/audio/generations` | Async; poll `GET /v1/audio/generations/{task_id}`. |
| Speech / transcribe / translate | `POST /v1/audio/speech`, `/transcriptions`, `/translations` | Sync binary or multipart. Do not poll these. |
| Realtime | `GET /v1/realtime` | WebSocket (`wss://…/v1/realtime?model=`). Never HTTP POST. |
| Embeddings / rerank / moderations | `/v1/embeddings`, `/v1/rerank`, `/v1/moderations` | Request/response per capability page. |
| Visible model list | `GET /v1/models` | Token-scoped, read-only discovery. |
| Token usage | `GET /api/usage/token/` | Token-scoped quota and limits. |
| Account models | `GET /api/user/models` | Account-scoped enabled models. Needs an account credential. |
| Account profile | `GET /api/user/self` | Account-scoped profile and quota. Needs an account credential. |

Use `Authorization: Bearer $CANGYUAN_API_KEY` for `/v1/*` and `/api/usage/token/`. Use an **account credential** for `/api/user/*`. A relay `sk-` token returns `401` on `/api/user/*`.

`POST /v1/files` and `/v1/fine-tunes` are unimplemented (501). Do not call them.

Kling `/kling/v1` and Jimeng `/jimeng` are compatibility paths, not the unified media contract. Prefer `/v1/videos` and `/v1/images/*` unless the user explicitly needs the vendor path.

## Chat Example

```python
import os
from openai import OpenAI

client = OpenAI(
    base_url=f"{os.environ['CANGYUAN_BASE_URL'].rstrip('/')}/v1",
    api_key=os.environ["CANGYUAN_API_KEY"],
)

response = client.chat.completions.create(
    model="MODEL_ID_FROM_V1_MODELS",
    messages=[{"role": "user", "content": "你好"}],
    stream=True,
)

for event in response:
    delta = event.choices[0].delta.content
    if delta:
        print(delta, end="", flush=True)
```

## Video Task Pattern

Use the target model page before choosing fields such as `duration`, `aspect_ratio`, `resolution`, `reference_image_urls`, `reference_videos`, or `first_image_url`. Different video models do not accept the same parameters.

If that page lists `/v1/videos/assets` or `face_mode`, follow it before submitting. Do not put a real-human-face HTTPS URL into the generation request unless the page allows that path.

```python
import os
import time
import requests

base_url = os.environ["CANGYUAN_BASE_URL"].rstrip("/")
headers = {
    "Authorization": f"Bearer {os.environ['CANGYUAN_API_KEY']}",
    "Content-Type": "application/json",
}

submit = requests.post(
    f"{base_url}/v1/videos",
    headers=headers,
    json={
        "model": "MODEL_ID_FROM_V1_MODELS",
        "prompt": "Replace with the requested prompt",
        "aspect_ratio": "16:9",
    },
    timeout=45,
)
submit.raise_for_status()
task = submit.json()
task_id = task.get("task_id") or task.get("id")
if not task_id:
    raise RuntimeError(f"No task ID returned: {task}")

while True:
    detail = requests.get(f"{base_url}/v1/videos/{task_id}", headers=headers, timeout=30)
    detail.raise_for_status()
    result = detail.json()
    status = str(result.get("status", "")).lower()
    if status == "completed":
        video_url = result.get("video_url") or (result.get("data") or [{}])[0].get("url")
        if not video_url:
            raise RuntimeError(f"Task completed without a result URL: {result}")
        print(video_url)
        break
    if status == "failed":
        raise RuntimeError(result.get("fail_reason") or result.get("error") or result)
    time.sleep(5)
```

Suggested poll interval is 5–10 seconds. Client total wait should be at least 30 minutes for video. On `completed`, return the URL as-is. Do not GET `/v1/videos/{id}/content` to pull bytes through this API — that path proxies the file via the origin. If the returned URL itself is a `/content` link, still treat it as an opaque URL for the end user.

Image (`async: true`) and audio generation use the same poll-don't-resubmit rule on their own `GET …/{task_id}` paths. Chat / Responses / Embeddings are not task IDs.

## Token Usage

```bash
curl -sS "$CANGYUAN_BASE_URL/api/usage/token/" \
  -H "Authorization: Bearer $CANGYUAN_API_KEY"
```

Read the returned quota, used amount, limits, and expiration. This is a single token's view. For account-wide models and profile, see account credentials below.

## Account-Level Access

- A relay `sk-` token (`CANGYUAN_API_KEY`) authenticates `/v1/*` and `/api/usage/token/`.
- An **account credential** authenticates `/api/user/*`. A relay `sk-` token returns `401` on those routes.

Obtain an account credential from the console (personal access token / `GET /api/user/token` while logged in), then send `Authorization: Bearer <credential>`.

```bash
ACCOUNT_TOKEN="personal access token"

curl -sS "$CANGYUAN_BASE_URL/api/user/self" -H "Authorization: Bearer $ACCOUNT_TOKEN"
curl -sS "$CANGYUAN_BASE_URL/api/user/models" -H "Authorization: Bearer $ACCOUNT_TOKEN"
```

Keep the account token secret. Never put it in browser code.

## Files And Reference Media

There is no general-purpose `POST /v1/files` upload API. Do not attempt to use it. `image_ids` and `file_id` are not this platform's ingest.

For reference images, videos, or audio, follow the target model page:

1. Default: a public HTTPS URL that can be fetched anonymously. Never assume a local filesystem path is accepted.
2. If the model page lists `POST /v1/videos/assets`, POST the public URL first (body includes that page's `model`). Read `asset_id` or `id`. Poll `GET /v1/videos/assets/{asset_id}?model=` until `status` is `Active` (create responses often omit `status`). Then send `asset://{asset_id}` in the generation request.
3. If the model page lists `face_mode` for real human faces, follow that page. Do not mix `face_mode` + HTTPS with `asset://` on the same request.
4. Without a real human face, public HTTPS in the fields the model page lists is enough.

## Failure Handling

- `401`: verify the key and base URL.
- `403`: check the token group and model visibility from `GET /v1/models`.
- `429`: wait and retry with backoff; do not fan out parallel retries.
- `501` on `/v1/files` or `/v1/fine-tunes`: those routes are unimplemented.
- Async task `failed`: preserve the task ID and `fail_reason` / `error`; do not duplicate an accepted task. Messages are platform-normalized and do not include channel or vendor names.
- Media validation error: reread the target model's parameter table, including assets / `face_mode`, instead of guessing field names.
