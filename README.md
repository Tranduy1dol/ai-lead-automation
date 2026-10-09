# AI Lead Intake & Qualification

## Problem

Businesses receive leads from forms and manually review,
classify and route them.

## Solution

This service automatically:

1. receives leads through an API
2. classifies them using an LLM
3. stores structured data
4. notifies the sales team

## Architecture

`POST /leads` → lead handler → lead service → OpenAI classifier → PostgreSQL repository → Discord notifier (when configured).
The service receives its classifier, repository, notifier, and logger through constructor injection.
OpenAI-specific HTTP and JSON handling lives in `internal/llm`; tests retain deterministic
`lead.ClassifierFunc` mocks and use mocked HTTP responses for the adapter.

## Configuration

Export the variables in `.env.example` before running `go run ./cmd/api`; the application
does not load `.env` files automatically. `DATABASE_URL`, `OPENAI_API_KEY`, and
`OPENAI_MODEL` are required. The selected model must support the Responses API and
Structured Outputs; `.env.example` uses `gpt-4o-mini`.

The classifier sends the lead's company and message to OpenAI with `store: false`.
It requires a structured response containing `category` (`sales`, `support`, `spam`,
or `other`), `priority` (`low`, `normal`, or `high`), and nonempty `summary` and
`recommended_action` strings. Requests have a 30-second timeout and respect caller
cancellation. API failures, refusals, incomplete responses, and invalid classifications
return an error; the service does not persist the lead on classification failure.

## Discord notifications

Set `DISCORD_WEBHOOK_URL` to a Discord incoming webhook URL to enable notifications;
leave it empty to preserve operation without notifications. Invalid URLs fail startup.
Treat the URL as a secret. Each notification includes the persisted lead ID, company,
category, priority, classification summary, and recommended action. Empty fields use
`(not provided)`; long fields are truncated to 900 UTF-8 bytes (including an ellipsis)
for Discord's embed limits. Full values remain in PostgreSQL. Mentions are disabled.

Delivery is **best-effort in this MVP**. The service persists first, then synchronously
attempts one notification with the caller's context and a five-second timeout. Failed
classification or persistence never triggers a notification. Notification failures are
logged with the lead ID and do not change the successful creation response (`201`).
The adapter requests send confirmation (`wait=true`), rejects non-2xx responses, and
never retries or follows redirects, including on rate limits (`429`). See the
[Discord webhook API](https://docs.discord.com/developers/resources/webhook#execute-webhook).

A crash after persistence, caller cancellation, timeout, or Discord outage can lose a
notification; a timeout can also leave delivery uncertain. There is no durable queue,
Kafka, Redis, or outbox. Notification errors do not cause another insert, but the lead
API itself is not idempotent: manually repeating `POST /leads` can still create duplicates.

## Features

- REST API
- AI classification
- PostgreSQL persistence
- Webhook notifications
- Dockerized environment

## Tech Stack

- Go
- PostgreSQL
- Docker
- LLM API
