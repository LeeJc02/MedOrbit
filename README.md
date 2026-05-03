<p align="center">
  <img src="assets/logo.png" alt="MedOrbit logo" width="180" />
</p>

# MedOrbit

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)](go.mod)
[![Python](https://img.shields.io/badge/Python-3.12+-3776AB.svg)](python/runtime/requirements.txt)
[![CI](https://github.com/LeeJc02/MedOrbit/actions/workflows/ci.yml/badge.svg)](https://github.com/LeeJc02/MedOrbit/actions/workflows/ci.yml)
[![Release](https://github.com/LeeJc02/MedOrbit/actions/workflows/release.yml/badge.svg)](https://github.com/LeeJc02/MedOrbit/actions/workflows/release.yml)

MedOrbit is an open-source clinical triage demo for drug interaction and medication-safety workflows. It combines a Go HTTP gateway, JWT/RBAC protection, Postgres-backed audit replay, and a Python gRPC runtime that produces evidence-backed agent responses.

The repository still uses `ddi` as an internal compatibility identifier for the Go module, protobuf package, environment variables, database defaults, and generated stubs. The public project and UI brand is MedOrbit.

## Features

- Go gateway with HTTP JSON APIs, JWT authentication, doctor-role RBAC, Swagger UI, and static demo UI.
- Python gRPC runtime with deterministic retrieval, evidence enforcement, draft generation, and health checks.
- Tenant-scoped audit logging and replay through Postgres plus runtime audit events.
- Offline and Docker-backed evaluation harness for agent quality gates.
- Dockerfiles for the gateway and runtime services.
- GitHub Actions CI and tag-triggered release publishing for binaries and GHCR images.

## Architecture

```mermaid
flowchart LR
  browser["Browser / curl"] --> gateway["Go gateway :8080"]
  gateway --> auth["JWT + doctor RBAC"]
  auth --> audit["Postgres audit log"]
  auth --> runtime["Python gRPC runtime :50051"]
  runtime --> graph["Agent graph"]
  graph --> evidence["Evidence + claims"]
  runtime --> runtimeAudit["Runtime audit trail"]
```

## Request Flow

```mermaid
sequenceDiagram
  participant Client as Browser or curl
  participant Gateway as Go gateway
  participant Audit as Postgres audit
  participant Runtime as Python runtime
  Client->>Gateway: POST /v1/session/run with JWT
  Gateway->>Gateway: Validate JWT and doctor role
  Gateway->>Audit: Write gateway.request
  Gateway->>Runtime: RunSession gRPC
  Runtime->>Runtime: Retrieve evidence and build claims
  Runtime-->>Gateway: Evidence-backed response
  Gateway->>Audit: Write gateway.response
  Client->>Gateway: POST /v1/session/replay
  Gateway->>Audit: Read same-tenant audit events
  Gateway-->>Client: Replay events
```

## CI/CD Flow

```mermaid
flowchart LR
  push["Push or pull request"] --> tests["Go tests + Python tests + evals + smoke"]
  tag["Push v* tag"] --> releaseTests["Full test suite"]
  releaseTests --> binaries["Gateway binaries"]
  releaseTests --> images["GHCR Docker images"]
  binaries --> ghRelease["GitHub Release"]
  images --> ghcr["GitHub Container Registry"]
```

## Quick Start

Install Python dependencies in your runtime/test environment:

```bash
conda run -n ddi-agent python -m pip install -r python/runtime/requirements.txt
```

Start MedOrbit locally:

```bash
make run-local
```

Open the demo UI:

```text
http://localhost:8080
```

Run the smoke test:

```bash
make e2e-smoke
```

## API and Authentication

Swagger UI is served at:

```text
http://localhost:8080/docs
```

Primary endpoints:

- `GET /` serves the MedOrbit demo UI.
- `GET /docs` serves Swagger UI for `openapi/openapi.yaml`.
- `POST /v1/session/run` runs a triage session.
- `POST /v1/session/replay` replays same-tenant audit events.

Protected HTTP routes require an HS256 JWT signed with `DDI_JWT_SECRET`. Required claims are:

- `sub` or `user_id`
- `tenant_id`
- `roles`, including `doctor`
- `exp`

The local demo UI signs a doctor-role token with the development secret `dev-secret`. Use a different secret for any non-local deployment.

## Agent Evaluation

```bash
go test ./...
conda run -n ddi-agent python -m pytest python/runtime/tests -q
conda run -n ddi-agent python -m evals.run_agent_eval --mode offline --fail-on-threshold
DDI_EVAL_INTEGRATION=1 conda run -n ddi-agent python -m evals.run_agent_eval --mode integration --fail-on-threshold
./scripts/e2e_smoke.sh
```

The integration eval expects Docker Compose services to be available:

```bash
docker compose up -d postgres etcd minio milvus
```

## Docker

Build local images:

```bash
docker build -f docker/gateway.Dockerfile -t medorbit-gateway:local .
docker build -f docker/runtime.Dockerfile -t medorbit-runtime:local .
```

Release images are published by tag-triggered GitHub Actions:

- `ghcr.io/leejc02/medorbit-gateway:<tag>`
- `ghcr.io/leejc02/medorbit-gateway:latest`
- `ghcr.io/leejc02/medorbit-runtime:<tag>`
- `ghcr.io/leejc02/medorbit-runtime:latest`

## Release Artifacts

Pushing a `v*` tag creates a GitHub Release with gateway binaries for:

- `linux-amd64`
- `linux-arm64`
- `darwin-amd64`
- `darwin-arm64`

Example:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DDI_HTTP_ADDR` | `:8080` | Go gateway listen address |
| `DDI_GRPC_ADDR` | `127.0.0.1:50051` | Runtime address used by the gateway |
| `DDI_RUNTIME_GRPC_ADDR` | `[::]:50051` | Python runtime listen address |
| `DDI_JWT_SECRET` | `dev-secret` | HS256 JWT signing secret |
| `DDI_PG_DSN` | `postgres://ddi:ddi@localhost:5432/ddi?sslmode=disable` | Gateway audit Postgres DSN |
| `DDI_AUDIT_ENABLED` | `true` | Enables Postgres audit logging |
| `DDI_EVAL_INTEGRATION` | unset | Set to `1` to run Docker-backed evals |

## Project Structure

- `cmd/gateway`: Go gateway entrypoint.
- `internal/httpapi`: HTTP request and response handlers.
- `internal/middleware`: JWT and RBAC middleware.
- `internal/orchestrator`: Gateway service orchestration and audit replay.
- `internal/runtimeclient`: gRPC client for the Python runtime.
- `python/runtime`: Python gRPC runtime, agent graph, tests, and evals.
- `openapi`: Public HTTP API contract.
- `web`: Demo UI and Swagger UI shell.
- `assets`: Logo and favicon.
- `docker`: Gateway and runtime image definitions.
- `scripts`: Local setup, protobuf generation, and smoke test scripts.

## Security Notes

- Do not use the default `dev-secret` outside local development.
- Keep `.env`, certificates, keys, tokens, and generated eval reports out of Git.
- This demo is not a medical device and should not be used for real clinical decision-making without independent validation, governance, and compliance review.
- Avoid storing real patient data in local audit logs unless your environment is configured for that data.

## License

MedOrbit is released under the [MIT License](LICENSE).

---

# MedOrbit 中文说明

MedOrbit 是一个开源临床分诊演示项目，面向药物相互作用和用药安全场景。它包含 Go HTTP 网关、JWT/RBAC 鉴权、Postgres 审计回放，以及生成有证据约束回答的 Python gRPC runtime。

仓库内部仍保留 `ddi` 作为兼容性标识，用于 Go module、protobuf package、环境变量、数据库默认值和生成代码。对外项目名称和界面品牌统一为 MedOrbit。

## 核心能力

- Go 网关提供 HTTP JSON API、JWT 鉴权、医生角色 RBAC、Swagger UI 和演示前端。
- Python gRPC runtime 提供确定性检索、证据约束、草稿生成和健康检查。
- Postgres 网关审计日志支持同租户回放，runtime 也记录本地审计事件。
- 支持离线评测和 Docker 依赖服务下的集成评测。
- 提供 gateway/runtime Dockerfile，以及 GitHub Actions CI/CD。

## 一键启动

安装 Python 依赖：

```bash
conda run -n ddi-agent python -m pip install -r python/runtime/requirements.txt
```

启动本地服务：

```bash
make run-local
```

访问：

```text
http://localhost:8080
```

运行冒烟测试：

```bash
make e2e-smoke
```

## 架构和流程

浏览器或 curl 请求进入 Go gateway，先经过 JWT 和医生角色校验，再写入 Postgres 审计日志，并通过 gRPC 调用 Python runtime。Runtime 执行检索和 agent graph，返回带证据的 claims、草稿、风险等级和追问。回放接口只读取同一 `tenant_id` 下的审计事件，避免跨租户访问。

## 测试和评测

```bash
go test ./...
conda run -n ddi-agent python -m pytest python/runtime/tests -q
conda run -n ddi-agent python -m evals.run_agent_eval --mode offline --fail-on-threshold
DDI_EVAL_INTEGRATION=1 conda run -n ddi-agent python -m evals.run_agent_eval --mode integration --fail-on-threshold
./scripts/e2e_smoke.sh
```

集成评测需要先启动 Docker Compose 依赖：

```bash
docker compose up -d postgres etcd minio milvus
```

## 发布产物

推送 `v*` tag 后，Release workflow 会创建 GitHub Release，上传 Go gateway 多平台二进制，并推送两个 GHCR 镜像：

- `ghcr.io/leejc02/medorbit-gateway:<tag>`
- `ghcr.io/leejc02/medorbit-runtime:<tag>`

## 安全说明

- 非本地环境不要使用默认 `dev-secret`。
- `.env`、证书、密钥、token 和评测报告不应提交到 Git。
- 该项目是演示系统，不是医疗器械；真实临床使用前需要独立验证、治理和合规审查。
- 本地审计日志不要写入真实患者数据，除非运行环境已经满足对应数据保护要求。

## 许可证

MedOrbit 使用 [MIT License](LICENSE) 发布。
