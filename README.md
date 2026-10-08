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

`POST /leads` → lead handler → lead service → OpenAI classifier → PostgreSQL repository.
The service receives its classifier and repository through constructor injection.
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
