# KVolt Documentation ⚡

Welcome to the official documentation for KVolt **v2.1** (production-promising).

## Basics
-   **[Getting Started](getting_started.md)**: Installation, Quick Start, and HTTPS. Use `kvolt.Default()`.
-   **[CLI Guide](cli.md)**: Use the `kvolt` command line tool.
-   **[Routing](router.md)**: Radix tree router, Groups, and Static files.
-   **[Context API](context.md)**: The heart of every request (`BindJSON`, `Query`).

## Core Features
-   **[Middleware](middleware.md)**: Logger, Recovery, RequestID, Timeout, JWT / JWTOptional, Rate Limiter, Tenant, RBAC, Idempotency, Audit, Metrics, CORS, Secure, Gzip.
-   **[OpenTelemetry](otel.md)**: optional `middleware/otel` tracing.
-   **[Structured Errors](errors.md)**: `pkg/httperr` + automatic JSON mapping.
-   **[gRPC](grpc.md)**: HTTP + gRPC in one process.
-   **[Configuration](configuration.md)**: Manage secrets and settings.
-   **[Caching](caching.md)**: High-performance in-memory cache (LRU cap).
-   **[Validation](validation.md)**: Struct validation requests.
-   **[Authentication](authentication.md)**: JWT helpers and middleware.
-   **[Session Authentication](session.md)**: Stateful session management.
-   **[Templates](templates.md)**: HTML rendering.
-   **[WebSockets](websockets.md)**: Real-time communication.
-   **[Server-Sent Events](sse.md)**: Streaming events to the browser.
-   **[Background Jobs](queue.md)**: Async task processing (memory + Redis).
-   **[Task Scheduler](scheduler.md)**: Cron jobs for recurring tasks.
-   **[Dependency Injection](dependency_injection.md)**: Simple IoC container.
-   **[Logging](logging.md)**: Structured JSON logging.
-   **[Swagger Docs](swagger.md)**: OpenAPI auto-generation.

## Testing & Quality
-   **[Testing Guide](testing.md)**: Unit testing with `pkg/test` and `pkg/testkit`.
-   **[Benchmarks](benchmarks.md)**: Reproducible microbenchmarks (`scripts/bench.ps1`).
-   **[Developer Workflow](developer_workflow.md)**: Lint, test, and run your app (fmt, vet, test helpers, CLI).
-   **[Local quality checks](report_card.md)**: fmt / vet / lint locally (Go Report Card site is sunset).

## Packages
-   `pkg/config`, `pkg/logger`, `pkg/validator`, `pkg/httperr`
-   `pkg/cache` (memory + Redis), `pkg/queue` (memory + Redis), `pkg/session`
-   `pkg/di`, `pkg/auth`, `pkg/scheduler`, `pkg/swagger`
-   `pkg/test`, `pkg/testkit`