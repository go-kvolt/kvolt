# Makefile for KVolt — run Go Report Card–style checks locally

.PHONY: fmt vet lint report test

# Format code (same as goreportcard gofmt check)
fmt:
	go fmt ./...

# Run go vet (same as goreportcard govet check)
vet:
	go vet ./...

# Run all tests
test:
	go test ./...

# Lint: fmt + vet. Install extra tools for full report-card parity (see below).
lint: fmt vet
	@echo "--- checking ineffassign ---"
	@command -v ineffassign >/dev/null 2>&1 && ineffassign ./... || echo "ineffassign not installed (go install github.com/gordonklaus/ineffassign@latest)"
	@echo "--- checking misspell ---"
	@command -v misspell >/dev/null 2>&1 && misspell -error . || echo "misspell not installed (go install github.com/client9/misspell/cmd/misspell@latest)"
	@echo "--- checking staticcheck ---"
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed (go install honnef.co/go/tools/cmd/staticcheck@latest)"

# Same checks as Go Report Card (run from repo root: make report)
report: fmt vet test
	@echo "--- ineffassign ---"
	@command -v ineffassign >/dev/null 2>&1 && ineffassign ./... || true
	@echo "--- misspell ---"
	@command -v misspell >/dev/null 2>&1 && misspell -error . || true
	@echo "--- staticcheck ---"
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || true
	@echo "Done. Fix any reported issues to match goreportcard.com."
