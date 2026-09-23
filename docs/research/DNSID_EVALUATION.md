# DNSid Evaluation: `draft-ihsanullah-dnsid` and `did:nfd`

Evaluation of the IETF Internet-Draft **"DNS-Anchored Durable Identity for AI Agents (DNSid)"** against the `did:nfd` method and this resolver. This document captures what the draft specifies, how its authors position it relative to DIDs, where it does and does not intersect with `did:nfd`, and what — if anything — we should do about it.

**Draft under review:** [`draft-ihsanullah-dnsid-01`](https://datatracker.ietf.org/doc/draft-ihsanullah-dnsid/), N. Ihsanullah (Innovation Labs / Identity Digital). Individual submission, Standards Track intent. Last updated 2026-06-28, expires 2026-12-31.

**Evaluated:** 2026-09-23, against repo HEAD `80a71b8`.

---

## Table of Contents

1. [Summary](#1-summary)
2. [What DNSid Specifies](#2-what-dnsid-specifies)
3. [How the Draft Positions Itself Against DIDs](#3-how-the-draft-positions-itself-against-dids)
4. [Side-by-Side with did:nfd](#4-side-by-side-with-didnfd)
5. [Why did:nfd Cannot Be a DNSid Identifier](#5-why-didnfd-cannot-be-a-dnsid-identifier)
6. [Where It Genuinely Fits](#6-where-it-genuinely-fits)
7. [Maturity and Landscape](#7-maturity-and-landscape)
8. [Recommendation](#8-recommendation)
9. [Repo Touchpoints Today](#9-repo-touchpoints-today)
10. [References](#10-references)

---

## 1. Summary

**DNSid does not compete with `did:nfd`, and nothing in this resolver needs to change today.**

The two specifications answer different questions. DNSid answers *"which accountable legal entity is responsible for this agent, including agents since retired and keys since rotated?"* — it calls this Layer 1. `did:nfd` answers *"what are the current cryptographic keys, service endpoints and profile data for this subject?"* — DNSid calls that Layer 2, and explicitly lists DIDs as a Layer 2 standard it sits beneath rather than replaces.

Three findings:

- **Three hard blockers** prevent `did:nfd` from being used as a DNSid identifier in the base profile (§5). None is an oversight we could work around; all three are deliberate.
- **One genuine opportunity**: the draft names `lr=algorand:AGENT_ADDR_BASE32` as an illustrative lifecycle-log method and creates an IANA registry for log-method bindings, explicitly modelled on W3C DID method specs. Algorand is a strong substrate for the durability and timestamp requirements. It is *not* a strong fit for the append-only-event and inclusion-proof requirements without new work (§6).
- **One live window**: the draft was brought to the IETF DNSOP list for DISPATCH on 2026-09-15, and a feedback call opened on 2026-09-17. If we want the `algorand` log method registered or a DID URL permitted in the key-reference tags, that is the cheapest point of influence and it is open now.

---

## 2. What DNSid Specifies

Each agent is assigned a **Fully Qualified Domain Name** under a domain controlled by its accountable entity — e.g. `billing-agent.acme-corp.example`, or `agent-7f3a9c2b.example` where the label is an opaque generated identifier. The FQDN is capped at 246 octets so the derived owner name fits the 253-octet DNS limit.

A single TXT record is published at `_dnsid.<agent-fqdn>` carrying `;`-separated tag/value pairs:

| Tag | Required | Meaning |
|-----|----------|---------|
| `v` | yes | Version. MUST be `DNSid1`, MUST be the first tag |
| `gi` | yes | Governance identifier — the registrant domain of the accountable entity |
| `ek` | yes | HTTPS URL for the accountable entity's record-signing keys, in JWKS format |
| `ku` | yes | HTTPS URL for the agent's operational signing keys, in JWKS format |
| `lr` | yes | Log reference — `method:location`, locating the agent's lifecycle log entry |
| `su` | yes | HTTPS URL for registration and revocation status |
| `sg` | yes | Base64url entity signature over the rest of the record |
| `fl` | no | Policy flags; `mtls` and `logchk` currently defined |
| `ka` | no | Maximum operational signing key age |
| `cu` | no | Capabilities URI — an AGENTS.md or Agent Card document |

A delegated example from §5.2, where a contractor runs the agent but Acme is accountable for it:

```
_dnsid.payroll.contractor.example. 300 IN TXT (
    "v=DNSid1; gi=acme-corp.example; fl=mtls,logchk;"
    "ek=https://acme-corp.example/dnsid/entity-keys.json;"
    "ku=https://payroll.contractor.example/dnsid/op-keys.json;"
    "lr=scitt:https://log.acme-corp.example/entries/77c1;"
    "su=https://payroll.contractor.example/dnsid/status;"
    "sg=<base64url-encoded-signature>"
)
```

**Signature.** `sg` covers the canonical record string: every tag except `sg`, sorted alphabetically by tag name, joined with `;`, no whitespace, encoded US-ASCII. It MUST be produced with the accountable entity's key from `ek`; the operational key at `ku` MUST NOT be used. Implementations MUST support ES256 and SHOULD support Ed25519 (JOSE/OKP EdDSA per RFC 8037).

**Keys.** Both `ek` and `ku` serve a JWK Set over HTTPS with a valid TLS certificate presenting a `dNSName` SAN. Each endpoint MUST contain **exactly one** current signing key; superseded keys MUST NOT be served there. Every key carries `kid` and `alg`.

**Lifecycle log.** `lr` is method-prefixed and points into an append-only log. §8.2 requires append-only entries, cryptographic inclusion proofs, verifiable timestamps, verifier accessibility, and durability "independently of the log operator's continued participation". Event entries introducing or rotating keys must preserve enough public key material for historical verification independent of the current JWKS endpoint.

**States.** `PENDING → PROVISIONING → VERIFYING → ACTIVE → {RETIRED, REVOKED}`. Key rotation and delegation happen within ACTIVE. RETIRED is graceful — "signed work products produced before retirement are not invalidated solely by retirement". REVOKED requires a reason code: `keyCompromise`, `policyViolation`, `superseded`, or `cessationOfOperation`.

**Verification (§9.1)** is six steps: resolve the TXT record and require exactly one RR → validate version and required tags → fetch the `ek` JWKS and verify `sg` → establish TLS (or mTLS if flagged) to the agent FQDN itself → the *bilateral binding check*, retrieving the ISSUANCE event from the log and confirming both the entity signature and the operational-key countersignature match the TXT record → query `su` and confirm state is ACTIVE. §9.2 adds optional historical verification via log inclusion proofs when `logchk` is set.

---

## 3. How the Draft Positions Itself Against DIDs

The draft is explicit and non-adversarial. Appendix A:

> "DIDs and Verifiable Credentials ([DID-CORE], [VC-DATA-MODEL]) establish cryptographic binding and attestation at Layer 2. DNSid differs: it operates at Layer 1 (durable accountable ownership), not Layer 2 (cryptographic identity binding). A DID identifies a subject; a DNSid identifies the accountable entity responsible for an agent. These are complementary: an agent's DID might reference its DNSid as part of its broader identity assertion, or a verifier might use DNSid accountability to evaluate the trustworthiness of a DID issuer. They address different layers of the identity stack and do not compete."

And on blockchain specifically:

> "Blockchain-based identity systems (including some DID methods) provide decentralized attestation and immutability. DNSid is blockchain-agnostic: the lifecycle log may be instantiated on a blockchain, but DNSid does not require or assume blockchain deployment. The two can coexist: a blockchain DID method might anchor agents and reference their DNSid assertions; a DNSid might record its lifecycle log on-chain."

This is consistent throughout. §1.3 lists DIDs among the standards DNSid "sits beneath" (alongside SPIFFE, OAuth, OIDC, SCIM, NGAC, MCP, A2A and AGENTS.md). §3.1's seven-layer model puts DIDs/VCs/PKI at Layer 2 and DNSid alone at Layer 1, arguing that existing standards cluster at Layers 2–7 and leave Layer 1 unserved. §3.2's non-goals explicitly exclude issuing or managing verifiable credentials.

The gap DNSid claims is **temporal**, not cryptographic: determining which entity owned an agent *at a past time*, after credentials, keys and even domains have changed. That is a question `did:nfd` does not currently answer either — our resolver returns the current state of an NFD and nothing about its history.

---

## 4. Side-by-Side with `did:nfd`

| | DNSid | `did:nfd` |
|---|---|---|
| **Question answered** | Which entity is accountable for this agent, now and historically? | What are this subject's current keys, services and profile? |
| **Layer (draft's own model)** | 1 | 2 |
| **Identifier** | FQDN in the ICANN root, e.g. `billing-agent.acme-corp.example` | `did:nfd:<name>.algo`, regex `^([a-z0-9]{1,27}\.){1,2}algo$` (`internal/did/resolver.go:38`) |
| **Also accepts** | — | An Algorand address, for reverse resolution (`internal/did/resolver.go:446-455`) |
| **Trust root** | WebPKI + the lifecycle log; DNSSEC strengthens it but is not assumed | Algorand consensus + the NFD registry contract |
| **Key encoding** | JWKS over HTTPS (RFC 7517) | multibase `z` + base58btc, multicodec `0xed01` / `0xec01` in the DID Document (`internal/did/keys.go:20-21`) |
| **Algorithms** | ES256 MUST, Ed25519 SHOULD | Ed25519 only (Algorand account keys) |
| **Key cardinality** | Exactly one current key per endpoint | Several co-equal: `#owner`, `#algo-0..N`, `#x25519-owner`, plus `u.keys` |
| **Key agreement** | None | `#x25519-owner`, Ed25519→X25519 (`internal/did/keys.go:74-87`) |
| **Service endpoints** | None (Layer 7 is out of scope) | `#web`, `#profile`, `#deposit`, socials, plus user-defined `u.service` |
| **Revocation** | REVOKED + reason code, served live from `su` | Boolean deactivation, no reason code (`internal/did/resolver.go:126-145`) |
| **History** | Append-only log, ISSUANCE and KEY_ROTATION events, inclusion proofs | None exposed. `i.timeCreated` / `i.timeChanged` only (`internal/did/resolver.go:321-330`) |
| **Discovery** | DNS TXT lookup | HTTP resolver, Universal Resolver driver |
| **Status** | Individual I-D, no WG adoption | Implemented, deployed, v1.0 method spec |

The two are close to **disjoint in what they carry**. DNSid has no key agreement, no service endpoints and no profile data. `did:nfd` has no revocation reason codes, no key-age policy, no status service and no append-only event history. Neither is a subset of the other, and neither duplicates the other's work.

---

## 5. Why `did:nfd` Cannot Be a DNSid Identifier

Three blockers, all in the base profile, all deliberate.

**1. `.algo` is not in the ICANN root.** DNSid requires a real TXT record resolvable at `_dnsid.<agent-fqdn>`, and verification step 4 requires establishing TLS *to the agent FQDN itself* and validating a certificate under local WebPKI policy. An `.algo` name satisfies neither: there is no authoritative DNS zone to publish into and no CA will issue for it. §14 further reserves the `_dnsid` underscore label under RFC 8552, which presumes ordinary DNS delegation.

**2. `gi` MUST be a domain name.** §5.4 leaves no ambiguity:

> "In this specification the 'gi' value MUST be a domain name (the registrant domain) in lowercase ASCII… Non-domain identifiers for the accountable entity (for example, a URI into an external registry, a Decentralized Identifier, or a Verifiable Credential subject) are reserved for future profiles and MUST NOT be used in the base profile."

A `did:nfd` URI is named as exactly the thing prohibited. This is a reservation, not a rejection — but it is normative today.

**3. Key transport and cardinality mismatch.** DNSid mandates JWKS over HTTPS with exactly one current key per endpoint. `did:nfd` publishes multibase-encoded Ed25519 inside a DID Document and deliberately models *several* co-equal keys — `#owner` plus `#algo-0..N` derived from `v.caAlgo`, reflecting NFD's multi-account identity model (method spec §9.3). Converting is mechanically easy; reconciling "exactly one current key" with a method whose whole point is linking multiple accounts is not. Separately, an Algorand-only key set is Ed25519, which DNSid only SHOULDs — a relying party implementing just the MUST (ES256) could not verify us.

---

## 6. Where It Genuinely Fits

Four hooks, ranked by how much they are worth.

### 6.1 The `lr=algorand:` log-method binding — the real opportunity

§5.5 lists three illustrative log methods, and one of them is ours:

```
lr=algorand:AGENT_ADDR_BASE32
lr=ctlog:https://log.example.com/agents/entries/12345
lr=scitt:https://transparency-service.example/entry/abc123
```

§8.1 sets out how a binding becomes real:

> "Concrete log bindings SHOULD be documented as separate specification documents, analogous to W3C DID method specifications. Each binding document defines the method name, reference syntax, entry format, proof format, and query interface for a specific log technology."

§14 creates a **DNSid Log Method Registry** under Specification Required policy. Registering `algorand` is an open, well-defined path, and the draft's authors have already gestured at it.

Where Algorand is genuinely strong is §8.2's hardest requirement — *"entries persist independently of the log operator's continued participation"* — plus independently verifiable timestamps. Most candidate logs (a vendor-run SCITT service, a CT log) satisfy durability only as long as someone keeps paying for it. A public chain does not have that failure mode.

The gaps are real and should not be glossed:

- **NFD is mutable global state plus box storage, not an event log.** `i.timeChanged` overwrites; it does not append. §8.2 wants append-only entries with stable positions.
- **There are no lifecycle events.** §8.3 requires ISSUANCE countersigned by the initial operational key, and KEY_ROTATION authorized by the *previous* operational key rather than the entity key. NFD records neither, and the bilateral binding check in §9.1 step 5 depends entirely on the ISSUANCE event existing.
- **No inclusion proofs are exposed.** Algorand can produce them, but nothing in this repo implements or serves them — and the resolver speaks only to algod, with no indexer, so an NFD application's transaction history is not reachable from this service at all.

A conformant binding therefore needs either a purpose-built event-log application or an indexer-backed reconstruction of NFD history, with a specification pinning down entry encoding, canonicalization and proof format. Both are real projects. Neither is a small change, and neither should start before the draft has a home in a working group.

### 6.2 The NFD owner key as the `sg` signer

An Algorand account key is Ed25519, and DNSid SHOULDs Ed25519 for `sg` via RFC 8037. An NFD owner can therefore sign their own `_dnsid` record with precisely the key `did:nfd` already publishes as `#owner` — no new key material, no new custody problem, and the same proof-of-ownership property the method spec relies on in §11.1.

Be precise about the limit: the normative verification path is still "fetch the JWKS from `ek`". A DID Document is *corroborating* evidence here, not conformant discovery. It would become conformant only if a future profile permitted a DID URL in `ek`/`ku` — which is the specific, narrow extension worth asking the authors for.

### 6.3 `v.domain` as the bridge

`v.domain` feeding the `#web` `LinkedDomains` service (`internal/did/resolver.go:250-278`) is the only domain-tied element anywhere in the method. It is also the whole coexistence picture: an NFD whose owner has a verified real domain is exactly the party who *can* publish a conformant DNSid record — at that domain, not at `.algo`. The `did:nfd` document and the `_dnsid` record then describe the same operator from two directions, and neither spec needs to change.

Current treatment is an opaque URL string: no host parsing, no reverse check, no bidirectional proof. Verification happens off-repo inside the NFD platform. If DNS-anchored attestation ever matters to us, this is the property to build on.

### 6.4 `cu` capabilities URI

`cu` points at an AGENTS.md or Agent Card document. That maps cleanly onto a future user-defined NFD property, alongside the existing `u.*` set. Noted for completeness; no proposal attached.

---

## 7. Maturity and Landscape

`draft-ihsanullah-dnsid-01` is an **individual submission with no working group adoption**, Standards Track intent, expiring 2026-12-31. It was brought to the IETF DNSOP list on 2026-09-15 by Gabriel Kuettel (Identity Digital) and taken up as a DISPATCH item by Benno Overeinder on 2026-09-17, who invited the list to say both where the work should proceed and what it thinks of the draft. Possible homes mentioned include DNSOP, DAWN and ART.

It is not alone. As of 2026-09 at least three mutually incompatible individual drafts occupy this space:

| Draft | Mechanism | Status |
|---|---|---|
| `draft-ihsanullah-dnsid` | `_dnsid` TXT record, entity-signed | Individual, DISPATCH pending |
| `draft-nemethi-aid-agent-identity-discovery` | `_agent` TXT record | Individual |
| `draft-mozleywilliams-dnsop-dnsaid` | SVCB with DNS-SD/DNSSEC/DANE, TXT only as fallback | Individual; states it is "not endorsed by the IETF" and has "no formal standing" |

They disagree on the record type, the owner-name convention and the trust model. `draft-mozleywilliams-dnsop-dnsaid` argues specifically *against* TXT, on the grounds that SVCB was designed for service discovery and supports a `TargetName` distinct from the queried domain — which TXT cannot express.

The practical consequence: **do not ship a `v=DNSid1` parser against a wire format that three active drafts disagree about.** The cost of being early here is a versioned, published parser we would have to support; the benefit is close to zero until one approach is adopted.

---

## 8. Recommendation

**No changes to the resolver or the method spec.** DNSid neither threatens `did:nfd` nor offers anything we can consume in its current form.

Revisit if any of these happen:

- DNSOP, DAWN, ART or another WG adopts the draft.
- Anyone proposes an `algorand` binding for the DNSid Log Method Registry — at which point we should be involved rather than reacting.
- A customer asks for an NFD-backed agent identity, which would make §6.2's shared-key story concrete.
- A future profile relaxes §5.4 to permit non-domain `gi`, or permits a DID URL in `ek`/`ku`.

The DNSOP feedback window is open now and is the cheapest available point of influence. Two asks are worth making, both narrow and both in the authors' own interest:

1. **Register `algorand` as a log method**, or at minimum keep the example and point at a binding document — the draft already uses it as an illustration, so this costs them nothing and gives the registry a public-ledger entry that satisfies §8.2's durability clause better than a hosted service can.
2. **Permit a DID URL in `ek`/`ku` in a future profile.** §5.4 already reserves DIDs for `gi`; extending the same reservation to the key-reference tags is consistent with the draft's stated complementarity and would let a DID Document serve as conformant key discovery rather than mere corroboration.

Neither ask requires the base profile to change, which is what makes them plausible.

---

## 9. Repo Touchpoints Today

For the record, as of HEAD `80a71b8`: **there is no DNS code in the DID path at all.** No TXT record parsing, no `.well-known/did.json`, no `did-configuration.json`, no DNSSEC, no DNS client. The only outbound network calls this service makes are to algod.

`v.domain` → the `#web` `LinkedDomains` service is the sole domain-tied element (§6.3 above).

This evaluation did surface dead DNS plumbing, since removed. `internal/nfd/fetch.go` was originally lifted from TxnLab's CoreDNS plugin and still carried the plugin's DNS path: an unused `FetchNfdDnsVals` interface method, an unexported and unreferenced `newNfdFetcher`, an `AlgoXyzIp` field the exported constructor never set, and — because that field was always empty — a synthesized fake A record written into `properties.UserDefined["dns"]` on every fetch. Nothing in `internal/did` read that key, so it was inert rather than harmful, but it kept `github.com/coredns/coredns` as a direct module dependency for the sake of a single debug log line. All of it is gone, and the v3 contract check that guards `v.blueskydid` was preserved.

One behavioral difference is worth recording rather than glossing over. The v3 gate used to fire on `u.dns` as well as `v.blueskydid`, so a live pre-v3 NFD carrying a `u.dns` value failed DID resolution outright with `ErrNFdIncompatible`. The narrowed gate only checks `v.blueskydid`, so those NFDs now resolve normally. This is reachable in practice: box reads are filtered by the DID property list (which never asked for `u.dns`), but global state is not filtered, so a V1-era NFD storing `u.dns` in global state would land in `properties.UserDefined["dns"]` and trip the old gate. Every other path is unchanged — the synthesized `dns` key that the old code wrote on the remaining branches was never read by `internal/did` and never serialized into a response, so no DID Document byte moves because of its removal.

---

## 10. References

### The draft

- [`draft-ihsanullah-dnsid` — datatracker](https://datatracker.ietf.org/doc/draft-ihsanullah-dnsid/)
- [`draft-ihsanullah-dnsid-01` — text](https://www.ietf.org/archive/id/draft-ihsanullah-dnsid-01.txt)
- [`draft-ihsanullah-dnsid-01` — HTML](https://www.ietf.org/archive/id/draft-ihsanullah-dnsid-01.html)
- [DNSOP list: "Feedback for draft-ihsanullah-dnsid"](http://www.mail-archive.com/dnsop@ietf.org/msg33626.html), 2026-09-15

### Competing drafts

- [`draft-mozleywilliams-dnsop-dnsaid` — DNS for AI Discovery](https://datatracker.ietf.org/doc/draft-mozleywilliams-dnsop-dnsaid/)
- `draft-nemethi-aid-agent-identity-discovery`

### Normative references the draft depends on

- [RFC 7517](https://www.rfc-editor.org/rfc/rfc7517) — JSON Web Key (JWK)
- [RFC 7518](https://www.rfc-editor.org/rfc/rfc7518) — JSON Web Algorithms
- [RFC 8037](https://www.rfc-editor.org/rfc/rfc8037) — CFRG curves for JOSE
- [RFC 8552](https://www.rfc-editor.org/rfc/rfc8552) — Scoped interpretation of DNS underscore names
- [RFC 6698](https://www.rfc-editor.org/rfc/rfc6698) — DANE TLSA

### In-repo

- [`docs/DID_NFD_METHOD_SPEC.md`](../DID_NFD_METHOD_SPEC.md) — §9.3 multi-account identity model, §10 key pipeline, §11 security considerations
- [`docs/research/REVERSE_RESOLUTION.md`](./REVERSE_RESOLUTION.md) — §1 standards landscape, including `.well-known/did-configuration`
- [`docs/research/README.md`](./README.md) — DID method comparison, including `did:web`
