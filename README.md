<div align="center">

# 🔍 request-inspector

**See exactly what your client sends — every method, every path, one JSON dump.**

[![CI](https://github.com/v01dst/request-inspector/actions/workflows/ci.yml/badge.svg)](https://github.com/v01dst/request-inspector/actions/workflows/ci.yml)
![License](https://img.shields.io/badge/license-MIT-8A2BE2)
![Go](https://img.shields.io/badge/go-1.27-00ADD8?logo=go&logoColor=white)
![Deps](https://img.shields.io/badge/dependencies-stdlib%20only-00c853)
![Tests](https://img.shields.io/badge/tests-passing-brightgreen)

`debug webhooks` · `inspect headers` · `verify proxies` · `single binary`

</div>

---

## ✨ Features

- **🪞 Any method, any path** — POST, PUT, DELETE, anything → full JSON echo
- **🌐 Proxy-aware IPs** — honors `X-Forwarded-For` chains automatically
- **🍪 Cookie parsing** — headers *and* structured cookie map
- **🧬 Binary detection** — upload a file, get `<binary N bytes>` instead of mojibake
- **📦 Body size guard** — configurable cap, clean `413` responses
- **🪶 Pure stdlib** — zero Go dependencies, 9 MB static binary

## 🚀 Quick Start

```bash
git clone https://github.com/v01dst/request-inspector
cd request-inspector
go build -o inspector .
PORT=3000 ./inspector
```

Or with Docker:

```bash
docker build -t inspector .
docker run -p 3000:3000 inspector
```

## 📡 Usage

```bash
curl -X POST "http://localhost:3000/anything?x=1" \
  -H 'x-forwarded-for: 203.0.113.5' \
  -d 'hi'
```

```json
{
  "method": "POST",
  "path": "/anything",
  "query": "x=1",
  "proto": "HTTP/1.1",
  "host": "localhost:3000",
  "remoteIp": "203.0.113.5",
  "tls": false,
  "headers": {
    "content-length": "2",
    "content-type": "application/x-www-form-urlencoded",
    "user-agent": "curl/8.18.0",
    "x-forwarded-for": "203.0.113.5"
  },
  "bodyBytes": 2,
  "body": "hi",
  "userAgent": "curl/8.18.0",
  "cookies": {},
  "timestamp": "2026-09-04T04:43:07.903Z"
}
```

| Route     | Behavior                     |
|-----------|------------------------------|
| `/health` | `{"status":"ok"}`            |
| `/`       | Service info                 |
| `/*`      | Full request echo (any verb) |

## ⚙️ Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT`   | `3000`  | Listen port |

## 🧱 Tech Stack

| Layer     | Tech                  |
|-----------|-----------------------|
| Language  | Go 1.27 (stdlib only) |
| Server    | net/http              |
| Testing   | go test + httptest    |
| Packaging | Docker (multi-stage)  |
| CI        | GitHub Actions        |

---

<div align="center">

Built with ⚡ by **v01dst**

[![GitHub](https://img.shields.io/badge/github-v01dst-181717?logo=github)](https://github.com/v01dst)
[![Discord](https://img.shields.io/badge/discord-9p.1-5865F2?logo=discord&logoColor=white)](https://discord.com/users/9p.1)

*Project 007 / 99 — The Loop*

</div>
