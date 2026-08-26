# KVolt Documentation ⚡

Welcome to the official documentation for KVolt **v2.0** (production).

## Basics
-   **[Getting Started](getting_started.md)**: Installation, Quick Start, and HTTPS. Use `kvolt.Default()`.
-   **[CLI Guide](cli.md)**: Use the `kvolt` command line tool.
-   **[Routing](router.md)**: Radix tree router, Groups, and Static files.
-   **[Context API](context.md)**: The heart of every request (`BindJSON`, `Query`).

## Core Features
-   **[Middleware](middleware.md)**: Logger, Recovery, RequestID, Timeout, JWT, Rate Limiter, CORS, Secure, Gzip.
-   **[gRPC](grpc.md)**: HTTP + gRPC in one process.
-   **[Configuration](configuration.md)**: Manage secrets and settings.
-   **[Caching](caching.md)**: High-performance in-memory cache (LRU cap).
-   **[Validation](validation.md)**: Struct validation requests.
-   **[Authentication](authentication.md)**: JWT helpers and middleware.
-   **[Session Authentication](session.md)**: Stateful session management.
-   **[Templates](templates.md)**: HTML rendering.
-   **[WebSockets](websockets.md)**: Real-time communication.
-   **[Background Jobs](queue.md)**: Async task processing.
-   **[Task Scheduler](scheduler.md)**: Cron jobs for recurring tasks.
-   **[Dependency Injection](dependency_injection.md)**: Simple IoC container.
-   **[Logging](logging.md)**: Structured JSON logging.
-   **[Swagger Docs](swagger.md)**: OpenAPI auto-generation.

## Testing & Quality
-   **[Testing Guide](testing.md)**: Unit testing with `pkg/test` and `pkg/testkit`.
-   **[Developer Workflow](developer_workflow.md)**: Lint, test, and run your app (fmt, vet, test helpers, CLI).
-   **[Go Report Card locally](report_card.md)**: Run Report Card–style checks (fmt, vet, lint) on your machine.

## Packages
-   `pkg/config`: Configuration loader.
-   `pkg/validator`: Struct validation.
-   `pkg/logger`: Structured logging.
-   `pkg/testkit`: Test utilities.