# v0.1.0 release audit and checklist

Audit date: 2026-10-11. Base revision: `eebc528`; release preparation changes are
uncommitted. Scope: all 27 tracked files, including application code, tests,
migrations, dependencies, container configuration, and documentation; all six
reachable commits were included in the credential-pattern scan.

## Readiness

Ready for review as a local portfolio demonstration. The mock workflow passed
end to end, including real PostgreSQL persistence and restart recovery. No broken
core workflow was observed in the executed checks. This does not establish live
provider compatibility, qualification accuracy, production security, or throughput.
No business features or application architecture were changed for this release.

## Implementation inventory

| Area | Implemented behavior | Verification |
| --- | --- | --- |
| HTTP API | `GET /health`; `POST /leads`; JSON success/error responses | Route/handler tests; both routes exercised by Compose demo |
| Validation | Trimming, required name/email/message, email syntax, field lengths, 1 MiB body cap, unknown/trailing JSON rejection | Source inspection; tests cover normalization, missing name, error mapping; not every boundary has a regression test |
| Orchestration | Validate → classify → persist → optionally notify | Service tests cover ordering and dependency failures |
| Mock classifier | Fixed sales/normal classification and recommendation; no network call | Unit test and Compose demo with API key empty |
| OpenAI classifier | Responses request, structured schema, cancellation, 30-second limit, response validation, refusal/error handling | Mock HTTP transport tests only; no live API call |
| Repository | Parameterized insert of lead and classification; generated ID/time; 3-second query limit | Disposable PostgreSQL integration test; demo compares all fields against SQL |
| Migrations | Embedded ordered SQL, transaction, history, advisory lock | Integration tests cover rollback, concurrent runners, repeated application preserving data; Compose startup passed |
| Discord adapter | Optional webhook after persistence, five-second limit, no retries/redirects, mentions disabled, bounded fields | Local HTTP server tests and service tests; no real Discord delivery |
| Runtime | Startup database ping, JSON logging, signal-driven shutdown | Source inspection; Compose start/stop passed; forced shutdown under load untested |
| Packaging | Multi-stage image, unprivileged scratch runtime, read-only filesystem, loopback ports, persistent volume | Compose build/start/demo/restart/cleanup passed |

The README previously lacked an explicit problem/value statement, verification
status, and one-command self-checking demo. These are now supplied along with an
interface-to-adapter Mermaid diagram. Existing manual setup and migration guidance
remain in place.

## Verification record

Executed on Linux amd64 with `go1.27.1-X:nodwarf5`, Docker Engine 29.8.2, and
Docker Compose 5.6.0. Commands below omit the local RTK output wrapper.

| Command/check | Result |
| --- | --- |
| `go test ./...` | Passed; PostgreSQL integration test is opt-in and skipped in this run |
| `go vet ./...` | Passed |
| `go build ./...` | Passed |
| `RUN_POSTGRES_TESTS=1 go test ./... -count=1` | Passed, including a new disposable PostgreSQL 17.6 container |
| `python3 -m py_compile scripts/demo.py` | Passed |
| `python3 scripts/demo.py` | Passed: build/start/migrate, health, HTTP 201, classification, all stored fields, persistence after restart, cleanup |
| `git diff --check` | Passed |
| Ignore rules | `.env`, `.env.local`, `bin/api` ignored; `.env.example` tracked; Python cache ignored |
| Credential review | No real credentials identified by source review or pattern scan across 6 commits / 50 unique blobs |

Docker required elevated socket access in the audit environment. The build used
cached layers and the classic builder because the buildx plugin was absent; this
is not a verified cold build or cross-platform build. The demo's unique project
was `lead-demo-6df7761fbdd7`; its containers, network, and database volume were
removed successfully. Build images/cache may remain.

## Configuration and security review

- `.env.example` uses a visibly local-only PostgreSQL password and blank OpenAI
  and Discord credentials. No real API keys are hardcoded. Test credentials are
  placeholders or disposable local database passwords.
- `.gitignore` covers environment variants and local binaries. Python cache is
  now excluded for the demo. `.dockerignore` allows only build inputs and excludes
  tests; environment files are outside the build context.
- The credential scan checked common OpenAI key, AWS access key, private key,
  and Discord webhook patterns without printing secret values. It is a heuristic,
  not proof of absence: unreachable Git objects, remote refs not present locally,
  and arbitrary secret formats were not scanned.
- SQL uses parameters. Client responses hide internal errors. OpenAI error bodies
  and Discord secret URLs are excluded from adapter error messages. Database
  errors are logged without a general redaction layer; restrict log access and
  do not treat logs as guaranteed free of connection details or user content.
- OpenAI receives company/message, not the separate name/email fields. Free text
  may still contain personal information. Discord receives company and generated
  classification content. `store: false` is a request setting, not a claim about
  all provider retention behavior.
- Compose publishes only to loopback. The native server listens on all interfaces.
  There is no authentication, rate limiting, or TLS termination. PostgreSQL uses
  a bootstrap superuser and local unencrypted connections in the sample setup.
- The webhook destination is operator-controlled and accepts arbitrary HTTP(S)
  URLs. Use a trusted HTTPS destination for real notifications; do not expose this
  configuration to untrusted users.
- Server request headers have a deadline; body reads have a size cap but no read
  deadline. Public deployment requires appropriate connection/body timeouts.
- No dependency vulnerability scanner, penetration test, or container vulnerability
  scan was run. Image tags are versioned but not digest-pinned.

## Outstanding issues and intentional limits

| Priority / scope | Issue | Recommended disposition |
| --- | --- | --- |
| Before publication | No license file or license decision is recorded | Owner should choose terms or explicitly retain all rights; do not invent a license |
| Before claiming live integrations | Real OpenAI classification and Discord delivery remain unverified | Either retain the explicit mock-tested label or run a separately authorized live check |
| Before public deployment | No access controls, TLS, rate limits, body deadline, restricted DB role, or retention/deletion policy | Keep v0.1.0 local; address these in a separately scoped deployment review |
| Reliability limit | Notifications can be lost after persistence; requests wait for notification attempt | Document best-effort delivery; no durable outbox/retry claim |
| API limit | No idempotency or read/list/update/delete endpoints | Retrying POST can duplicate leads; inspect persistence with SQL |
| Test gap | Validation boundary cases lack exhaustive regression coverage; no CI workflow | Add focused boundary tests and CI in future maintenance work |
| Operational limit | Health is liveness only; migration rollback/down and checksums are absent | Apply migrations before startup; inspect logs; back up data before upgrades |
| Evaluation gap | No semantic accuracy, latency, throughput, cost, or business outcome measurements | Do not publish benchmark or ROI claims |

These missing capabilities are outside the first release scope, not unfinished
implementations promised by this release. The current mock always returns the same
classification even for support or spam inquiries.

## Release checklist

- [x] Inspect implementation, callers, tests, migrations, and packaging.
- [x] Document problem, intended value, stack, features, API, configuration, and limits.
- [x] Show service interfaces and infrastructure adapters in Mermaid.
- [x] Provide a self-checking, isolated demo without paid API calls or notifications.
- [x] Verify SQL persistence and persistence after database/API restart.
- [x] Run Go tests, vet, build, PostgreSQL integration tests, and Compose smoke test.
- [x] Review committed credentials, ignore rules, and example environment values.
- [x] Distinguish local/mock verification from unverified live behavior.
- [ ] Owner reviews the final diff and accepts the documented release scope.
- [ ] Owner decides licensing before presenting the repository as reusable open source.
- [ ] Preview README Mermaid rendering on the hosting platform.
- [ ] After any further code changes, rerun verification on the chosen release revision.
- [ ] Manually commit approved changes, create `v0.1.0`, and publish release notes.

Suggested release description: “First portfolio release: validated HTTP lead
intake, classifier/repository/notifier interfaces, PostgreSQL persistence and
migrations, optional OpenAI and Discord adapters, and a verified mock-mode Compose
demo. Live provider behavior and production deployment are outside the verified scope.”

No commit, tag, push, or publication was performed during this preparation.
