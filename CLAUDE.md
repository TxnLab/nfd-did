# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

W3C DID Core v1.0 compliant resolver for the `did:nfd` method. Resolves decentralized identifiers backed by NFD (Non-Fungible Domains) on the Algorand blockchain into DID Documents. Runs as a standalone HTTP service or as a DIF Universal Resolver driver.

## Build & Run

```bash
# Build (requires the jsonv2 build tag)
go build -tags=goexperiment.jsonv2 -o did-resolver ./cmd/did-resolver

# Run (defaults to mainnet via Nodely, port 8080)
./did-resolver

# Docker (note: Dockerfile expects to be built from a parent context that copies this repo as did/)
docker build -t did-nfd-resolver .
```

Environment variables: `LISTEN` (`:8080`), `ALGOD_URL` (`https://mainnet-api.4160.nodely.dev`), `ALGOD_TOKEN`, `REGISTRY_ID` (`760937186` mainnet), `CACHE_TTL` (`5m`).

## Testing

```bash
go test ./...                      # all tests
go test ./internal/did -v          # DID resolver tests
go test ./internal/nfd -v          # NFD fetcher tests
go test ./internal/did -run TestResolve_BasicDocument -v  # single test
```

All tests use mocks (`mockNfdFetcher` implementing `nfd.NfdFetcher`) — no blockchain access needed.

## Architecture

Three-layer design with an HTTP server on top:

**`internal/nfd/`** — Blockchain data layer. `NfdFetcher` interface abstracts Algorand algod queries. `fetch.go` handles NFD lookup (registry box SHA256 hash → app ID → global state + box data), parallel fetching via `syncutil.WaitGroup`, and property extraction into the `Properties` struct (Internal/UserDefined/Verified maps). `misc.go` has NFD name validation.

**`internal/did/`** — DID resolution layer. `NfdDIDResolver` interface (`resolver.go`) validates DID strings via regex, calls `NfdFetcher.FetchNfdDidVals`, and constructs W3C-compliant DID Documents. Key responsibilities:
- Cryptographic key pipeline (`keys.go`): Algorand address → Ed25519 public key → multibase (base58btc + multicodec prefix 0xed01), and Ed25519 → X25519 derivation (multicodec prefix 0xec01) for KeyAgreement
- Auto-generating verification methods, services (web, profile, social media), and metadata
- Detecting deactivation states (expired, for-sale, unowned, explicit `u.deactivated`)
- Expirable LRU cache (50k entries, configurable TTL)

**`cmd/did-resolver/`** — HTTP server (stdlib `net/http`) exposing:
- `GET /1.0/identifiers/{did...}` — DID resolution (supports `application/did+json` and `application/did+ld+json`)
- `GET /1.0/properties` — Method metadata
- `GET /health` — Health check

## Key Conventions

- NFD properties use prefixed namespaces: `i.*` (internal/read-only), `u.*` (user-defined), `v.*` (verified) — prefixes are stripped when stored in `Properties` maps
- Verification method IDs: `#owner`, `#algo-0`..`#algo-N`, `#x25519-owner`
- Public keys use multibase base58btc encoding with multicodec prefixes
- Tests use `github.com/stretchr/testify` (assert/require)
- The Dockerfile builds from a parent context (`COPY did ./`) — image is `ghcr.io/txnlab/did-nfd-resolver`

## Important Docs

- `docs/DID_NFD_METHOD_SPEC.md` — Full W3C-style method specification
- `docs/DID_NFD_USER_GUIDE.md` — End-user guide with curl examples
- `universal-resolver/` — Universal Resolver driver config and Docker Compose setup
