# v0.1.0 release audit and checklist

Initial audit: 2026-10-11 at base revision `eebc528`. Day 8 hardening:
2026-10-12 at base revision `d6f1942`, plus the uncommitted CI/documentation changes.
The initial audit covered application code, tests, migrations, dependencies,
packaging, and documentation. Day 8 reviewed CI, test isolation, configuration,
and release claims; the refreshed credential scan covered all seven reachable
commits and the working files. Counts and results below describe this snapshot,
not a tagged release.

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

The initial preparation supplied README problem/value and verification statements,
a self-checking demo, and an interface-to-adapter Mermaid diagram. Day 8 adds CI
and updates verification records without changing application code or dependencies.

## Verification record

The automated checks below were invoked locally on 2026-10-12 on Linux amd64
with `go1.27.1-X:nodwarf5` and Docker Engine 29.8.2. Commands omit the local RTK
wrapper. These are local results, not evidence of a GitHub-hosted CI run.

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
| Credential pattern scan | No matches across 7 commits / 54 unique blobs and 30 working files; source/configuration review identified only test/demo placeholders |
| Workflow YAML/settings check | Passed local YAML parsing and assertions for triggers, permissions, Go version source, commands, blank credentials, and uncached integration opt-in; `actionlint` unavailable |
| `govulncheck ./...` | Incomplete: installed scanner uses Go 1.26 source-processing packages and cannot load this Go 1.27 project; no vulnerability verdict |

Docker required elevated socket access in the audit environment. The build used
cached layers and the classic builder because the buildx plugin was absent; this
is not a verified cold build or cross-platform build. The demo's unique project
was `lead-demo-8913fbed6099`; its containers, network, and database volume were
removed successfully. Build images/cache may remain.

Manual review covered test destinations, workflow secret handling, example
configuration, ignore rules, and stale documentation. README rendering, licensing,
and final release approval remain manual checklist items. Live OpenAI and Discord
checks were not performed; the automated adapter checks use mocks/local servers.

## CI design and pending hosted verification

- `.github/workflows/ci.yml` triggers on `push` and `pull_request`, with no branch
  or path filters. Two independent Ubuntu jobs separate Go checks from Docker
  integration failures; both have a ten-minute limit.
- Both use `actions/setup-go` with `go-version-file: go.mod`, avoiding a second
  Go version setting. This follows the action's
  [version-file support](https://github.com/actions/setup-go/blob/main/docs/advanced-usage.md).
- The Go job runs `go test ./...`, `go vet ./...`, and `go build ./...`.
  The PostgreSQL job runs `TestPostgresRepository` with `RUN_POSTGRES_TESTS=1`
  and `-count=1`; it reuses the existing container lifecycle rather than adding
  a service database or a configurable connection URL. Docker failures fail it.
- Repository permissions are `contents: read`; checkout does not persist its
  token. No repository/environment secrets are referenced. Application API keys,
  webhook URL, and database URL are explicitly empty. Steps do not dump environment
  variables or expanded Compose configuration. Only public disposable test
  passwords could appear in test failure diagnostics.
- GitHub-hosted execution, action/toolchain downloads, and hosted logs remain
  unverified until both jobs pass on the chosen release revision. Action versions
  use major tags, not immutable commit pins. The Compose demo and credential scan
  are local release checks and are not included in CI.

## Configuration and security review

- `.env.example` uses a visibly local-only PostgreSQL password and blank OpenAI
  and Discord credentials. No real API keys are hardcoded. Test credentials are
  placeholders or disposable local database passwords.
- `.gitignore` covers environment variants and local binaries. Python cache is
  now excluded for the demo. `.dockerignore` allows only build inputs and excludes
  tests; environment files are outside the build context.
- The credential scan checked common OpenAI key, AWS access key, private key,
  GitHub token, and Discord webhook patterns without printing secret values. It is a heuristic,
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
- The dependency vulnerability scan was attempted but could not complete because
  of scanner/toolchain incompatibility. No penetration test or container vulnerability
  scan was run. Image tags are versioned but not digest-pinned.
- PostgreSQL integration tests construct their URL only from the container they
  create on a random loopback port with tmpfs storage. They never consume
  `DATABASE_URL` or production configuration. Startup validation tests use an
  invalid database URL and fail before connecting; provider tests use HTTP mocks
  or local servers. The Compose demo overrides provider credentials and creates
  its own database volume.

## Outstanding issues and intentional limits

| Priority / scope | Issue | Recommended disposition |
| --- | --- | --- |
| Before publication | No license file or license decision is recorded | Owner should choose terms or explicitly retain all rights; do not invent a license |
| Before claiming live integrations | Real OpenAI classification and Discord delivery remain unverified | Either retain the explicit mock-tested label or run a separately authorized live check |
| Before public deployment | No access controls, TLS, rate limits, body deadline, restricted DB role, or retention/deletion policy | Keep v0.1.0 local; address these in a separately scoped deployment review |
| Reliability limit | Notifications can be lost after persistence; requests wait for notification attempt | Document best-effort delivery; no durable outbox/retry claim |
| API limit | No idempotency or read/list/update/delete endpoints | Retrying POST can duplicate leads; inspect persistence with SQL |
| Before release | CI workflow added but not executed on GitHub | Require both jobs to pass on the chosen release revision and inspect logs |
| Security verification gap | Installed vulnerability scanner cannot process Go 1.27 | Run a compatible scanner before claiming dependency vulnerability verification |
| Test gap | Validation boundary cases lack exhaustive regression coverage | Add focused boundary tests in future maintenance work |
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
- [x] Run local automated Go tests, vet, build, PostgreSQL integration tests, and Compose smoke test (2026-10-12).
- [x] Add push/PR CI with Go version from `go.mod` and an isolated PostgreSQL job.
- [x] Parse workflow YAML and inspect secret handling and test database isolation locally.
- [x] Review committed credentials, ignore rules, and example environment values.
- [x] Distinguish local/mock verification from unverified live behavior.
- [ ] Obtain passing GitHub Actions results for both jobs on the chosen release revision; inspect hosted logs for credential exposure.
- [ ] Complete a dependency vulnerability scan using a Go 1.27-compatible scanner.
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
