#!/usr/bin/env python3
"""Self-checking, no-paid-API Compose demo; removes only its own project."""

import datetime
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request
import uuid


ROOT = Path(__file__).resolve().parents[1]
PROJECT = "lead-demo-" + uuid.uuid4().hex[:12]
ENV = {
    **os.environ,
    "PORT": "0",
    "POSTGRES_PORT": "0",
    "POSTGRES_USER": "lead",
    "POSTGRES_PASSWORD": "local-demo-only",
    "POSTGRES_DB": "ai_lead_automation",
    "CLASSIFIER_MODE": "mock",
    "OPENAI_API_KEY": "",
    "OPENAI_MODEL": "",
    "OPENROUTER_API_KEY": "",
    "OPENROUTER_MODEL": "",
    "DISCORD_WEBHOOK_URL": "",
}
COMPOSE = [
    "docker", "compose", "--project-name", PROJECT,
    "--env-file", str(ROOT / ".env.example"),
    "-f", str(ROOT / "docker-compose.yml"),
]


def compose(*args, capture=False):
    return subprocess.run(
        [*COMPOSE, *args], cwd=ROOT, env=ENV, check=True,
        text=True, stdout=subprocess.PIPE if capture else None, timeout=600,
    ).stdout


def check(condition, message):
    if not condition:
        raise RuntimeError(message)


def ready():
    address = compose("port", "api", "8080", capture=True).strip()
    check(address.startswith("127.0.0.1:"), "API must be bound to localhost")
    base = "http://" + address
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            with urllib.request.urlopen(base + "/health", timeout=2) as response:
                check(json.load(response) == {"status": "ok"}, "Unexpected health response")
            return base
        except (urllib.error.URLError, TimeoutError):
            time.sleep(0.5)
    raise RuntimeError("API did not become healthy within 60 seconds")


def stored_lead(lead_id):
    # The integer check prevents response data becoming executable SQL.
    check(type(lead_id) is int and lead_id > 0, "Invalid generated lead ID")
    query = f"SELECT row_to_json(leads) FROM leads WHERE id = {lead_id}"
    return json.loads(compose(
        "exec", "-T", "db", "psql", "-X", "-U", "lead", "-d",
        "ai_lead_automation", "-v", "ON_ERROR_STOP=1", "-At", "-c", query,
        capture=True,
    ))


def verify_stored(created):
    stored = stored_lead(created["id"])
    expected = {key: value for key, value in created.items() if key != "classification"}
    expected.update(created["classification"])
    for value in (stored, expected):
        value["created_at"] = datetime.datetime.fromisoformat(
            value["created_at"].replace("Z", "+00:00")
        )
    check(stored == expected, "Persisted lead differs from the API response")


def main():
    print(f"Demo project: {PROJECT}", flush=True)
    print("Mock classification forced; OpenAI, OpenRouter, and Discord credentials disabled.", flush=True)
    try:
        compose("up", "--build", "-d")
        base = ready()
        payload = {
            "name": "Jane Doe", "email": "jane@example.com", "company": "Acme",
            "message": "We need invoice automation.",
        }
        request = urllib.request.Request(
            base + "/leads", data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json"}, method="POST",
        )
        with urllib.request.urlopen(request, timeout=10) as response:
            check(response.status == 201, "Lead submission did not return HTTP 201")
            created = json.load(response)
        check(all(created[key] == value for key, value in payload.items()), "Lead fields changed")
        check(created["classification"] == {
            "category": "sales", "priority": "normal", "summary": "Demo lead received.",
            "recommended_action": "Review the lead and contact the prospect.",
        }, "Unexpected mock classification")
        print(json.dumps(created, indent=2), flush=True)
        verify_stored(created)
        print("PASS: HTTP 201, mock classification, and all database fields match.", flush=True)
        compose("stop", "api", "db")
        compose("start", "db")
        compose("up", "-d", "api")
        ready()
        verify_stored(created)
        print("PASS: the lead survived a database/API restart.", flush=True)
        print("Notifications are disabled in this demo. When configured, Discord is attempted")
        print("after saving; failure is logged and still returns HTTP 201. No retry or outbox.")
    finally:
        print(f"Removing temporary demo project {PROJECT} and its volume.", flush=True)
        try:
            compose("down", "--volumes", "--remove-orphans")
        except (subprocess.SubprocessError, OSError):
            print(f"Cleanup failed. Run: docker compose -p {PROJECT} "
                  "--env-file .env.example -f docker-compose.yml down --volumes")
            raise


if __name__ == "__main__":
    main()
