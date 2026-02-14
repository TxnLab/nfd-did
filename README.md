# did:nfd — Decentralized Identifiers for Algorand NFDs

A [W3C DID Core v1.0](https://www.w3.org/TR/did-core/) compliant resolver for the `did:nfd` method. Resolves [Non-Fungible Domain (NFD)](https://app.nf.domains) names on the Algorand blockchain into DID Documents.

```
did:nfd:nfdomains.algo
```

Every NFD is automatically a DID. The resolver reads on-chain NFD properties and constructs a standards-compliant DID Document containing the owner's Ed25519 public key, derived X25519 key agreement key, verified linked accounts, service endpoints, and profile information — all sourced directly from the Algorand blockchain.

## Features

- **Zero setup** — If you own an NFD, you already have a DID. Owner key, authentication, and key agreement are derived automatically.
- **Human-readable** — `did:nfd:yourname.algo` instead of opaque identifiers like `did:key:z6Mk...`
- **Multi-account identity** — Provably link multiple Algorand wallets via verified addresses (`v.caAlgo`), with bidirectional reverse resolution.
- **Rich DID Documents** — Auto-generated services for web domains, social media (Twitter, Discord, Telegram, GitHub, LinkedIn, Bluesky), and NFD profiles.
- **Extensible** — Add custom service endpoints (`u.service`), additional verification methods (`u.keys`), controller overrides, and cross-chain identity links.
- **On-chain and verifiable** — Source of truth is the Algorand blockchain. No off-chain registry or separate anchoring transactions.

## Quick Start

```bash
# Build
go build -tags=goexperiment.jsonv2 -o did-resolver ./cmd/did-resolver

# Run (defaults to Algorand mainnet via Nodely on port 8080)
./did-resolver

# Resolve a DID
curl http://localhost:8080/1.0/identifiers/did:nfd:nfdomains.algo
```

### Docker

```bash
docker run -p 8080:8080 ghcr.io/txnlab/did-nfd-resolver:latest
```

Or build locally:

```bash
docker build -t did-nfd-resolver .
docker run -p 8080:8080 did-nfd-resolver
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN` | `:8080` | HTTP listen address |
| `ALGOD_URL` | `https://mainnet-api.4160.nodely.dev` | Algorand algod API URL |
| `ALGOD_TOKEN` | (empty) | Algorand algod API token |
| `REGISTRY_ID` | `760937186` | NFD Registry application ID (mainnet) |
| `CACHE_TTL` | `5m` | Resolved document cache TTL (Go duration format) |

## API

| Endpoint | Description |
|----------|-------------|
| `GET /1.0/identifiers/{did}` | Resolve a DID to a DID Document |
| `GET /1.0/properties` | Method metadata and capabilities |
| `GET /health` | Health check |

The resolution endpoint supports content negotiation via the `Accept` header: `application/did+json` (default) or `application/did+ld+json`.

### Response Codes

| Code | Meaning |
|------|---------|
| 200 | Successful resolution |
| 400 | Invalid DID format |
| 404 | NFD not found on-chain |
| 410 | DID deactivated (expired, for-sale, unowned, or explicitly deactivated) |
| 500 | Internal error (e.g., algod unreachable) |

### Example Response

```bash
curl -s http://localhost:8080/1.0/identifiers/did:nfd:nfdomains.algo | jq .
```

```json
{
  "didDocument": {
    "@context": [
      "https://www.w3.org/ns/did/v1",
      "https://w3id.org/security/suites/ed25519-2020/v1",
      "https://w3id.org/security/suites/x25519-2020/v1"
    ],
    "id": "did:nfd:nfdomains.algo",
    "controller": "did:nfd:nfdomains.algo",
    "verificationMethod": [
      {
        "id": "did:nfd:nfdomains.algo#owner",
        "type": "Ed25519VerificationKey2020",
        "controller": "did:nfd:nfdomains.algo",
        "publicKeyMultibase": "z6Mkf5r..."
      }
    ],
    "authentication": ["did:nfd:nfdomains.algo#owner"],
    "assertionMethod": ["did:nfd:nfdomains.algo#owner"],
    "keyAgreement": [
      {
        "id": "did:nfd:nfdomains.algo#x25519-owner",
        "type": "X25519KeyAgreementKey2020",
        "controller": "did:nfd:nfdomains.algo",
        "publicKeyMultibase": "z6LShs..."
      }
    ]
  },
  "didResolutionMetadata": {
    "contentType": "application/did+json",
    "retrieved": "2025-12-01T12:00:00Z",
    "duration": 42
  },
  "didDocumentMetadata": {
    "deactivated": false,
    "nfdAppId": 12345
  }
}
```

## Universal Resolver Integration

This resolver can run as a driver for the [DIF Universal Resolver](https://github.com/decentralized-identity/universal-resolver). See [`universal-resolver/`](universal-resolver/) for Docker Compose configuration and integration instructions.

## DID Identifier Format

```
did:nfd:<nfd-name>.algo
```

Both root NFDs (`name.algo`) and single-segment NFDs (`segment.name.algo`) are supported. The name must be lowercase alphanumeric, with each label 1-27 characters.

**Pattern:** `^did:nfd:([a-z0-9]{1,27}\.){1,2}algo$`

## Customizing Your DID Document

Set user-defined properties on your NFD (via [app.nf.domains](https://app.nf.domains) or Algorand SDK transactions) to customize your DID Document:

| Property | Purpose |
|----------|---------|
| `u.service` | Custom service endpoints (JSON array) |
| `u.keys` | Additional verification methods (JSON array) |
| `u.controller` | Override DID controller |
| `u.alsoKnownAs` | Link to other identifiers (JSON array) |
| `u.deactivated` | Set to `"true"` to deactivate |
| `u.name`, `u.bio`, `u.avatar`, `u.banner` | Auto-generates `#profile` NFDProfile service |
| `u.twitter`, `u.discord`, `u.telegram`, `u.github`, `u.linkedin` | Auto-generates SocialMedia services |

Verified properties (`v.domain`, `v.caAlgo`, `v.blueskydid`, `v.avatar`, etc.) take priority over user-defined equivalents where applicable.

## Documentation

- **[DID NFD Method Specification](docs/DID_NFD_METHOD_SPEC.md)** — Full W3C-style method specification
- **[User Guide](docs/DID_NFD_USER_GUIDE.md)** — Detailed guide covering all features, property configuration, challenge-response authentication, and troubleshooting

## Development

**Requirements:** Go 1.25+

```bash
# Build (requires jsonv2 build tag)
go build -tags=goexperiment.jsonv2 -o did-resolver ./cmd/did-resolver

# Run all tests (no blockchain access needed — all tests use mocks)
go test ./...

# Run tests for a specific package
go test ./internal/did -v
go test ./internal/nfd -v
```

## License

Copyright (c) 2024-2026 TxnLab Inc. All rights reserved.
