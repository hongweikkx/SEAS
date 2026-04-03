# Repository Guidelines

## Project Structure & Module Organization
This is a Go service built with Kratos. Entry points live in `cmd/seas/`, shared application logic is under `internal/`, and reusable helpers live in `pkg/`. Generated API and protobuf artifacts are under `api/seas/v1/` and `internal/conf/`. Keep source edits in the `.proto` files, `wire.go`, and package code; regenerate derived files instead of editing them by hand.

## Build, Test, and Development Commands
- `make init`: installs the local codegen toolchain (`protoc-gen-go`, `wire`, Kratos generators).
- `make generate`: runs `go generate ./...` and `go mod tidy`.
- `make api`: regenerates API protobuf, gRPC, HTTP, and OpenAPI files from `api/**/*.proto`.
- `make config`: regenerates config protobuf code from `internal/**/*.proto`.
- `make build`: builds the service into `./bin/`.
- `make run`: starts the app with `kratos run`.
- `go test ./...`: runs the Go test suite. The repo currently has no committed `_test.go` files, so add tests alongside the package you change.

## Coding Style & Naming Conventions
Use standard Go formatting: tabs for indentation, `gofmt`/`go fmt` for formatting, and `go test` to verify compilation. Package names are short and lowercase (`biz`, `data`, `server`). Generated files use the existing `_gen.go`, `.pb.go`, and `_grpc.pb.go` naming. Prefer clear, domain-specific names for services and structs.

## Testing Guidelines
Place unit tests next to the code they cover using Go’s conventional `*_test.go` naming. Focus on package-level behavior in `internal/biz/`, `internal/service/`, and helper packages in `pkg/`. If you add generated code or interface changes, run `go test ./...` plus the relevant `make api` or `make config` target to confirm regeneration is clean.

## Commit & Pull Request Guidelines
History shows lightweight commits with labels like `feat: 网页版本` and `Phase 1`. Keep subject lines short and imperative, and use a prefix when it helps summarize scope. Pull requests should explain what changed, why it changed, and how you verified it. Include screenshots or sample requests only when the change affects HTTP/UI behavior.

## Configuration & Generated Artifacts
Do not commit local runtime files such as `configs/config.yaml`, `bin/`, or `logs/`. If you change proto definitions or Wire providers, regenerate the derived files and include the updated outputs in the same change.
