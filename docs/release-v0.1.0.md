# v0.1.0 — AI Lead Automation Portfolio MVP

## Overview

AI Lead Automation turns incoming inquiries into structured leads with a category,
priority, summary, and recommended next action. Each lead is saved in PostgreSQL,
with an optional Discord notification to support sales follow-up.

This first milestone demonstrates the complete intake workflow in a local
portfolio MVP. It provides a starting point for client-specific automation;
production deployment requires additional security and operational work.

## Features

- **Lead intake:** Go REST API with validated contact fields and JSON responses.
- **AI qualification:** OpenAI and OpenRouter adapters produce structured
  classifications; a deterministic mock mode supports demos without paid API access.
- **Persistence:** PostgreSQL storage with generated lead IDs, timestamps, and
  embedded database migrations.
- **Team notifications:** optional Discord messages with lead details and suggested
  next actions, sent after the lead is saved.
- **Local setup:** Docker Compose and a self-checking demo that verifies stored
  data and persistence after a database/API restart.
- **Automated checks:** unit tests, isolated PostgreSQL integration tests, and
  GitHub Actions for tests, vet, and build.

## Demo

[Watch the workflow](https://youtu.be/fwRQ8wB_IvA):
Lead Intake → AI Classification → PostgreSQL → Discord.

## Documentation

[Repository and setup guide](https://github.com/Tranduy1dol/ai-lead-automation)

## Verification record

### Automated checks

| Check | Recorded result |
| --- | --- |
| Go tests, vet, and build | Passed locally on 2026-10-13 |
| PostgreSQL integration | Passed locally on 2026-10-13 using an isolated, disposable container; no shared database |
| GitHub Actions | Go checks and PostgreSQL integration jobs passed on 2026-10-13 at `93082115bb349f0dcf1df2a31d59c026de78a3a8` |
| Docker Compose demo | The 2026-10-12 audit records successful startup, migrations, HTTP 201, SQL field checks, restart persistence, and cleanup; not rerun during final preparation |
| Documentation checks | `git diff --check` passed; existing demo link preserved |
| Credential review | No credential-pattern matches in 32 tracked working files or downloaded CI logs on 2026-10-13; example/test credentials are placeholders |

[View the verified CI run](https://github.com/Tranduy1dol/ai-lead-automation/actions/runs/38431999334).
Its test, vet, build, and PostgreSQL test steps all succeeded. These results precede
final documentation edits; the approved release commit still needs passing CI.
Provider tests use mocked HTTP responses or local servers, not live LLM requests.
Credential scanning is heuristic and does not guarantee the absence of all secret formats.

### Manual smoke tests — October 12, 2026

I tested a real OpenRouter request through the application and received HTTP `201`
with a structured classification. I also received a real Discord notification with
lead ID, company, category, priority, summary, and recommended action.

The observed classification identified an invoice automation and demo inquiry as
`sales` with `high` priority, summarized the request, and recommended assigning it
to sales and arranging a demonstration.

These are my manual observations, separate from automated CI results, and were not
independently reproduced during release preparation. The OpenAI adapter has automated
tests but remains unverified against the live OpenAI API. Classification accuracy,
reliability, production performance, and business outcomes have not been measured.
Human review remains necessary for generated recommendations.

## Current scope and future development

v0.1.0 focuses on lead intake, classification, persistence, and team notification.
The following are potential development areas, to be prioritized around client
requirements. They are not included in this release or committed to a delivery date.

| Development area | Current v0.1.0 behavior | Potential next step |
| --- | --- | --- |
| Secure deployment | Local MVP without authentication, rate limiting, TLS termination, or production hardening | Add access controls, deployment security, and operational monitoring |
| Reliable notifications | One synchronous, best-effort attempt; failures do not undo a saved lead and delivery can be lost | Introduce durable delivery, retries, and delivery tracking |
| Duplicate prevention | No request idempotency; repeated submissions can create duplicate leads | Add idempotency keys for safe retries |
| Sales workflow | API intake and SQL inspection; no lead management UI or CRM integration | Add lead review, search, and integration with a chosen CRM |
| Provider evaluation | Live OpenRouter smoke test; live OpenAI remains unverified | Verify OpenAI and evaluate classification quality using representative inquiries |
| Data lifecycle | Lead text may reach an LLM provider and generated content may reach Discord; no retention or deletion workflow | Define consent, retention, and deletion requirements |
