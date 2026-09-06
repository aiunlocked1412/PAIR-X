<!--
SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
SPDX-License-Identifier: Apache-2.0
-->

# nvpair-gateway (PAIR-X MVP)

`nvpair-gateway` is the first upstream-isolated PAIR-X extension. It exposes one local HTTP gateway, loads dynamic provider and capability definitions from JSON, checks provider health, and maps PAIR-X requests to custom HTTP APIs.

## Implemented routes

- `GET /health`
- `GET /v1/providers` (credential headers are never returned)
- `GET /v1/capabilities`
- `POST /v1/audio/speech`

The TTS route accepts the OpenAI speech shape (`model`, `input`, `voice`, `speed`) and applies the configured `requestMapping`. Exact placeholders such as `{{speed}}` preserve the original JSON type. Binary/audio responses are streamed back without buffering the complete response.

## Run

```bash
cp providers.example.json providers.json
go run . -config providers.json
```

The default listen address is `127.0.0.1:14322`. Until gateway authentication and cluster mTLS land, non-loopback listen addresses are rejected. Provider credentials and endpoint URLs remain only in the local configuration and are omitted from the provider-list API.

## Test

```bash
go test ./...
```

## MVP boundaries

This vertical slice is intentionally separate from PAIR core to keep upstream merges manageable. Broker supervision, node-to-node capability advertisements, persisted job tracking, weighted scheduling, additional OpenAI routes, and the desktop provider wizard are follow-up slices.
