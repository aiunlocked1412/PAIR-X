<!--
SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
SPDX-License-Identifier: Apache-2.0
-->

# PAIR-X — Universal Local AI Gateway

## 1. Project Overview

PAIR-X คือการ Fork โปรเจกต์ NVIDIA Personal AI Router (PAIR) เพื่อขยายจากระบบที่รองรับเฉพาะ LLM inference engine เช่น Ollama และ LM Studio ให้กลายเป็น **Universal Local AI Gateway**

เป้าหมายหลักคือ:

> โปรแกรมทั้งหมดในระบบเชื่อมต่อกับ PAIR-X เพียงจุดเดียว ส่วน PAIR-X เป็นผู้ค้นหา เลือก และกระจายงานไปยัง AI Service ที่เหมาะสมในแต่ละเครื่อง

ตัวอย่าง Service ที่ต้องรองรับ:

- LLM
- TTS
- STT
- Image Generation
- Video Generation
- Embedding
- Reranker
- MLX
- ComfyUI
- Custom HTTP API
- AI Service อื่นที่เพิ่มในอนาคต

---

# 2. Core Concept

PAIR เดิมมีแนวคิดหลักประมาณ:

```text
Application
     │
     ▼
    PAIR
     │
     ▼
Engine
├── Ollama
└── LM Studio
     │
     ▼
   Model
```

PAIR-X ต้องขยายเป็น:

```text
Application / Agent
        │
        ▼
     PAIR-X
   AI Gateway
        │
        ▼
 Capability Router
        │
 ┌──────┼────────┬─────────┐
 ▼      ▼        ▼         ▼
LLM    TTS      Image     Video
 │      │        │         │
 ▼      ▼        ▼         ▼
DGX    Mac      RTX       RTX
```

Application ไม่จำเป็นต้องรู้ว่า Service จริงอยู่เครื่องไหน

---

# 3. Primary Goal

ทุก Application ต้องสามารถตั้งค่าเพียง:

```env
AI_GATEWAY=http://pair.local
```

แทนการตั้งหลาย Endpoint เช่น:

```env
LLM_URL=http://192.168.1.10:8000
TTS_URL=http://192.168.1.20:8001
COMFY_URL=http://192.168.1.30:8188
MLX_URL=http://192.168.1.20:8080
```

Application ติดต่อ PAIR-X เพียงระบบเดียว

PAIR-X รับผิดชอบ:

1. Service Discovery
2. Node Discovery
3. Capability Discovery
4. Health Check
5. Routing
6. Load Balancing
7. Failover
8. Job Tracking
9. Provider Management
10. Request Forwarding

---

# 4. Architecture

```text
                    Applications

          ┌────────────┼────────────┐
          │            │            │
       Agents       Web Apps    Live Avatar
          │            │            │
          └────────────┼────────────┘
                       │
                       ▼

                ┌───────────────┐
                │    PAIR-X     │
                │  AI Gateway   │
                └───────┬───────┘
                        │
                Capability Router
                        │
        ┌───────────────┼─────────────────┐
        │               │                 │
        ▼               ▼                 ▼

   DGX Spark          Mac Mini          RTX PC
   Node A             Node B            Node C

   LLM                MLX               ComfyUI
   TTS                TTS               Image
   Embedding          STT               Video
```

---

# 5. Important Design Rule

PAIR-X ต้องไม่ผูก Provider เข้ากับ Hardware

ตัวอย่าง:

ผิด:

```text
TTS = Mac
LLM = DGX
Image = RTX
```

ถูก:

```text
Node
 └── Providers
       └── Capabilities
```

ตัวอย่าง:

```text
DGX
├── Ollama
│    └── llm
│
└── Qwen TTS
     └── tts


Mac
├── MLX
│    └── llm
│
└── Qwen TTS
     └── tts


RTX PC
└── ComfyUI
     ├── image
     └── video
```

ดังนั้น TTS สามารถมีพร้อมกันหลายเครื่องได้

---

# 6. Provider System

สร้าง Provider abstraction ใหม่

Provider ไม่ควรเป็น Closed Enum แบบ:

```text
ollama
lmstudio
mlx
tts
comfyui
```

ควรเป็น Dynamic Provider

ตัวอย่าง Data Model:

```json
{
  "id": "qwen-tts-main",
  "name": "Qwen3 TTS",
  "providerType": "custom-http",
  "baseUrl": "http://127.0.0.1:8000",
  "healthCheck": "/health",
  "capabilities": [
    "tts"
  ]
}
```

อีกตัว:

```json
{
  "id": "mlx-main",
  "name": "MLX Server",
  "providerType": "openai-compatible",
  "baseUrl": "http://127.0.0.1:8080/v1",
  "capabilities": [
    "llm"
  ]
}
```

---

# 7. Built-in Providers

PAIR-X ควรรักษา Provider เดิมไว้:

```text
Ollama
LM Studio
```

และเพิ่มระบบ:

```text
OpenAI Compatible Provider
Custom HTTP Provider
```

ไม่จำเป็นต้อง hard-code MLX, vLLM หรือ SGLang หาก API เหล่านั้นสามารถต่อผ่าน OpenAI-compatible Provider ได้

---

# 8. Custom Provider

UI ต้องสามารถ:

```text
+ Add Provider
```

แล้วกำหนด:

```text
Name

Provider Type

Base URL

Health Check URL

Capabilities

Authentication

Headers

Routes
```

ตัวอย่าง:

```text
Name:
AI UNLOCK TTS

Provider:
Custom HTTP

URL:
http://192.168.1.20:8000

Health:
/health

Capabilities:
[x] TTS
[x] Voice Clone
[x] Streaming
```

---

# 9. Capabilities

สร้าง Capability System ที่ไม่ผูกกับ Provider

Built-in capabilities:

```text
llm
tts
stt
embedding
rerank
image
video
vision
voice-clone
```

และต้องรองรับ custom capability เช่น:

```text
lipsync
background-removal
face-swap
upscale
music
```

เพื่อไม่ต้องแก้ Core ทุกครั้งที่เพิ่ม AI Service ใหม่

---

# 10. Capability Discovery

แต่ละ Node ต้องประกาศว่าให้บริการอะไร

ตัวอย่าง:

```json
{
  "node": "mac-mini",
  "providers": [
    {
      "id": "mlx",
      "capabilities": ["llm"]
    },
    {
      "id": "qwen-tts",
      "capabilities": [
        "tts",
        "voice-clone"
      ]
    }
  ]
}
```

DGX:

```json
{
  "node": "dgx-spark",
  "providers": [
    {
      "id": "ollama",
      "capabilities": ["llm", "embedding"]
    },
    {
      "id": "qwen-tts",
      "capabilities": ["tts"]
    }
  ]
}
```

---

# 11. Routing

เมื่อ request เข้ามา PAIR-X ต้องหา Node ที่:

```text
Online
+
Provider Healthy
+
มี Capability
+
รองรับ Model/Feature ที่ Request ต้องการ
+
มี Capacity
```

แล้วจึงเลือก Node

---

# 12. Load Balancing

ตัวอย่าง:

Mac:

```text
Qwen3-TTS
Load: 80%
```

DGX:

```text
Qwen3-TTS
Load: 20%
```

มี request:

```text
TTS Job #123
```

PAIR-X ควรเลือก:

```text
DGX
```

---

# 13. Multiple TTS Nodes

กรณี:

```text
PAIR-X
  │
  ├── Mac
  │    └── Qwen3-TTS
  │
  └── DGX
       └── Qwen3-TTS
```

ถ้ามี:

```text
TTS #1
TTS #2
TTS #3
TTS #4
```

สามารถกระจายเป็น:

```text
#1 → Mac
#2 → DGX
#3 → Mac
#4 → DGX
```

ขึ้นอยู่กับ Load

ไม่จำเป็นต้อง Round Robin อย่างเดียว

---

# 14. Job vs Job Splitting

PAIR-X Core ต้องแยกแนวคิดนี้ให้ชัดเจน

หนึ่ง Request:

```text
Generate TTS
```

ปกติจะถูกส่งไป Node เดียว

```text
PAIR-X
   ↓
DGX
```

PAIR-X ไม่ควรแบ่ง request เดียวข้ามเครื่องโดยอัตโนมัติใน MVP

---

# 15. Future: Job Splitter

ออกแบบ extension point ไว้สำหรับอนาคต:

```text
Long TTS
   │
   ▼
Job Splitter
   │
   ├── Segment 1
   ├── Segment 2
   ├── Segment 3
   └── Segment 4
          │
          ▼
       PAIR-X
       /     \
     Mac     DGX
       \     /
        Merge
```

แต่ Job Splitting ไม่ใช่ requirement ของ MVP

---

# 16. Gateway API

PAIR-X ควร expose API กลาง

OpenAI-compatible routes:

```text
POST /v1/chat/completions
POST /v1/embeddings

POST /v1/audio/speech
POST /v1/audio/transcriptions

POST /v1/images/generations
```

PAIR-X extensions:

```text
POST /v1/video/generations

POST /v1/tasks

GET /v1/providers

GET /v1/nodes

GET /v1/capabilities

GET /v1/jobs
```

---

# 17. TTS API

แนะนำให้ PAIR-X expose:

```http
POST /v1/audio/speech
```

Request:

```json
{
  "model": "qwen3-tts",
  "input": "สวัสดีครับ",
  "voice": "mor1",
  "speed": 1.0
}
```

Application ไม่ต้องระบุ Node

PAIR-X เป็นคนเลือก

---

# 18. Provider Adapter

Custom API แต่ละตัวอาจใช้ Request Format ไม่เหมือนกัน

ดังนั้นต้องมี Adapter Layer

```text
PAIR-X Standard Request

        ↓

Provider Adapter

        ↓

Actual Provider API
```

ตัวอย่าง PAIR-X:

```json
{
  "input": "สวัสดีครับ",
  "voice": "mor1"
}
```

API จริงอาจต้องการ:

```json
{
  "text": "สวัสดีครับ",
  "speaker": "mor1"
}
```

Adapter ต้องสามารถ Mapping ได้

---

# 19. Custom HTTP Mapping

Custom Provider ควรกำหนด:

```text
Method
Path
Headers
Request Mapping
Response Mapping
Response Type
Timeout
```

ตัวอย่าง:

```text
Method:
POST

Path:
/tts

Request:

{
  "text": "{{input}}",
  "speaker": "{{voice}}",
  "speed": "{{speed}}"
}
```

---

# 20. Response Types

Custom Proxy ต้องรองรับ:

```text
JSON
Binary
Audio
Image
Video
SSE
Streaming
```

สำคัญมากสำหรับ:

```text
TTS
STT
ComfyUI
Video Generation
LLM Streaming
```

---

# 21. Health Monitoring

ทุก Provider ต้องมีสถานะ:

```text
ONLINE
BUSY
DEGRADED
OFFLINE
```

Health Check เช่น:

```http
GET /health
```

PAIR-X ต้องตรวจเป็นระยะ

หาก Provider Offline:

```text
ไม่ควร Route งานไป Provider นั้น
```

---

# 22. Failover

ตัวอย่าง:

```text
TTS Request
     ↓
PAIR-X
     ↓
เลือก Mac
     ↓
Mac ERROR
     ↓
Retry
     ↓
DGX
```

ต้องกำหนด:

```text
maxRetries
retryableErrors
timeout
fallbackProvider
```

แต่ต้องระวัง request ที่ไม่ควรถูกทำซ้ำอัตโนมัติ

---

# 23. Job Tracking

ทุก Request ควรสร้าง Job

ตัวอย่าง:

```json
{
  "jobId": "job_123",
  "capability": "tts",
  "provider": "qwen-tts",
  "node": "dgx-spark",
  "status": "running",
  "startedAt": "...",
  "latency": null
}
```

เมื่อเสร็จ:

```text
QUEUED
RUNNING
COMPLETED
FAILED
```

---

# 24. Metrics

เก็บข้อมูล:

```text
Node Load
Active Jobs
Queue Length
Average Latency
Requests/minute
Error Rate
Provider Availability
```

สำหรับ Routing Decision

---

# 25. Scheduler

Scheduler ต้องสามารถใช้ข้อมูล:

```text
Capability Match
Model Match
Provider Health
Current Jobs
Queue Length
Latency
Hardware Capability
User Preference
```

คำนวณ Node Score

แนวคิด:

```text
score =
capability_match
+ model_match
+ availability
+ performance
- current_load
- queue_penalty
```

เลือก Node ที่ Score สูงที่สุด

---

# 26. Routing Policies

รองรับ:

```text
least-loaded
round-robin
lowest-latency
preferred-node
performance
```

ในอนาคตอาจมี:

```text
power-efficient
cost-aware
quality-first
```

---

# 27. Preferred Node

ควรตั้งค่าได้ เช่น:

```text
qwen3-tts

Preferred:
Mac

Fallback:
DGX
```

หรือ:

```text
GLM

Preferred:
DGX
```

PAIR-X จะเลือก Preferred ก่อนถ้าเครื่องพร้อม

---

# 28. Hardware Independence

PAIR-X ไม่ควรสนใจว่า Provider ใช้:

```text
CUDA
ROCm
MLX
CPU
NPU
```

PAIR-X สนใจเพียง:

```text
Provider Healthy?
Capability Available?
Can Handle Request?
```

---

# 29. Example Home AI Cluster

Target deployment:

```text
DGX Spark
128GB Unified Memory

Providers:
├── LLM
├── Embedding
└── TTS


Mac Mini

Providers:
├── MLX
├── TTS
└── STT


RTX 5090 PC

Providers:
├── ComfyUI
├── Image Generation
└── Video Generation
```

---

# 30. Application View

Application ต้องเห็นเพียง:

```text
AI Gateway
```

ไม่เห็น:

```text
DGX
Mac
5090
```

เช่น:

```javascript
const client = new OpenAI({
  baseURL: "http://pair.local/v1",
  apiKey: "local"
});
```

จากนั้น:

```text
chat.completions
```

PAIR-X → LLM

```text
audio.speech
```

PAIR-X → TTS

```text
images.generate
```

PAIR-X → Image Provider

---

# 31. Security

รักษาระบบ security เดิมของ PAIR ให้มากที่สุด

โดยเฉพาะ:

```text
Node Pairing
Cluster Authentication
mTLS
Secure Node Communication
```

Custom Provider credentials ต้องไม่ถูกประกาศออก network

เช่น:

```text
API Key
Bearer Token
Custom Headers
```

เก็บเฉพาะ Node เจ้าของ Provider

---

# 32. UI

เพิ่มหน้า:

```text
Providers
```

ตัวอย่าง:

```text
Providers

AI UNLOCK TTS

Node:
Mac Mini

Status:
ONLINE

Capabilities:
TTS
Voice Clone

Active Jobs:
2

Latency:
320 ms

[Test]

[Edit]

[Disable]
```

---

# 33. Add Provider UI

```text
+ Add Provider
```

Wizard:

```text
Step 1
Provider Type

○ OpenAI Compatible
○ Custom HTTP


Step 2
Connection

Name
Base URL
Authentication


Step 3
Capabilities

□ LLM
□ TTS
□ STT
□ Image
□ Video
□ Embedding
□ Custom


Step 4
Health Check


Step 5
Test Connection


Step 6
Save
```

---

# 34. Node Dashboard

ตัวอย่าง:

```text
DGX SPARK
ONLINE

Providers

Ollama
LLM
3 Active Jobs

Qwen3 TTS
TTS
1 Active Job
```

Mac:

```text
MAC MINI
ONLINE

MLX
LLM
0 Jobs

Qwen3 TTS
TTS
2 Jobs
```

---

# 35. Provider Test

ต้องมี:

```text
Test Connection
```

ผล:

```text
Connected

Latency:
18 ms

Health:
OK

Capabilities:
TTS

Streaming:
Supported
```

---

# 36. Backward Compatibility

สำคัญมาก

PAIR-X ต้องพยายามรักษาความเข้ากันได้กับ PAIR upstream

Ollama และ LM Studio เดิมต้องยังทำงาน

เป้าหมาย:

```text
PAIR-X = PAIR + Extensions
```

ไม่ใช่:

```text
PAIR-X = Rewrite PAIR
```

---

# 37. Upstream Strategy

Fork NVIDIA PAIR

ตั้ง:

```text
origin
→ PAIR-X repository

upstream
→ NVIDIA/Personal-AI-Router
```

พยายามแยก extension code ออกจาก core ให้มากที่สุด

เช่น:

```text
providers/
capabilities/
custom-proxy/
gateway/
```

เพื่อให้ merge upstream ในอนาคตง่าย

---

# 38. Proposed New Components

แนวทางเบื้องต้น:

```text
services/

nvpair-engine-manager/
nvpair-node-scanner/
nvpair-job-scheduler/

ollama-proxy/
lmstudio-proxy/

NEW:

nvpair-gateway/
nvpair-provider-manager/
nvpair-custom-proxy/
nvpair-capability-registry/
```

---

# 39. Provider