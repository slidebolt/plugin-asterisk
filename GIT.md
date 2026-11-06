# Git Workflow for plugin-asterisk

This repository contains the Slidebolt Asterisk Plugin. It translates between `asterisk-host` and Slidebolt: every device comes from the host's REST API, and it holds no call state or behavior of its own. It produces a standalone binary.

## Dependencies
- **Internal:**
  - `sb-contract`: Core interfaces and shared structures.
  - `sb-domain`: Shared domain models.
  - `sb-messenger-sdk`: Shared messaging interfaces.
  - `sb-runtime`: Core execution environment.
  - `sb-storage-sdk`: Shared storage interfaces.
  - `sb-testkit`: Testing utilities.
  - `sb-virtual`: Virtual device provider.
- **Runtime:** `asterisk-host`, reached through `ASTERISK_HOST_URL` and `ASTERISK_HOST_TOKEN`.
- **External:** 
  - `github.com/cucumber/godog`: BDD testing framework.

## Build Process
- **Type:** Go Application (Plugin).
- **Consumption:** Run as a background plugin service.
- **Artifacts:** Produces a binary named `plugin-asterisk`.
- **Command:** `go build -o plugin-asterisk ./cmd/plugin-asterisk`
- **Validation:** 
  - Validated through unit tests: `go test -v ./...`
  - Validated through BDD tests: `go test -v ./cmd/plugin-asterisk`
  - Validated by successful compilation of the binary.

## Pre-requisites & Publishing
As an Asterisk integration plugin, `plugin-asterisk` must be updated whenever the core domain, messaging, storage, or testkit SDKs are changed.

**Before publishing:**
1. Determine current tag: `git tag | sort -V | tail -n 1`
2. Ensure all local tests pass: `go test -v ./...`
3. Ensure the binary builds: `go build -o plugin-asterisk ./cmd/plugin-asterisk`

**Publishing Order:**
1. Ensure all internal dependencies are tagged and pushed.
2. Update `plugin-asterisk/go.mod` to reference the latest tags.
3. Determine next semantic version for `plugin-asterisk` (e.g., `v1.0.5`).
4. Commit and push the changes to `main`.
5. Tag the repository: `git tag v1.0.5`.
6. Push the tag: `git push origin main v1.0.5`.

## Update Workflow & Verification
1. **Modify:** Update the host client in `internal/hostapi` or the entity mapping in `internal/mapping`. Asterisk logic belongs in `asterisk-host`.
2. **Verify Local:**
   - Run `go mod tidy`.
   - Run `go test ./...`.
   - Run `go test ./cmd/plugin-asterisk` (BDD features).
   - Run `go build -o plugin-asterisk ./cmd/plugin-asterisk`.
3. **Commit:** Ensure the commit message clearly describes the Asterisk plugin change.
4. **Tag & Push:** (Follow the Publishing Order above).
