# SizeRight runbook

## Health endpoints
- `GET /healthz` → 200 if the process is alive. If not 200: restart the container.
- `GET /readyz` → 200 if Postgres is reachable, 503 if not. Load balancers use this.

## Alert: "SizeRight 5xx spike" (> 5 server errors in 15 min, checked every 5 min)
1. Splunk: `index=main sourcetype=sizeright level=ERROR | stats count by msg, err`
2. Pick a `request_id` from the alert and search it: `index=main request_id=<id>`
3. Match the error to a scenario below.

## Scenario: Postgres down
- Symptoms: readyz 503, POST /v1/sizings 500, log `save sizing failed ... connection refused`.
- Fix: `docker compose start db` (prod: check managed DB status page). App reconnects by itself, no restart.
- Verify: readyz 200, a POST returns 201.

## Scenario: Redis down
- Symptoms: log `redis: connection pool: failed to dial`, cache_hit always false. Requests still 201 (cache fails open).
- Fix: `docker start sizeright-redis`. No app restart needed.

## Scenario: bad deploy (panics / new 500s after a release)
- Symptoms: 500s with `{"error":"internal error"}`, log `panic recovered`, healthz still 200.
- Fix: roll back to the last good version (revert the commit / redeploy the previous image tag).
- Verify: the failing endpoint returns 200 again, 5xx panel back to 0.

## Scenario: many 429s
- Expected under abuse: per-user limit is `RATE_LIMIT_PER_MIN` (default 120).
- If a legitimate user is blocked: confirm in Splunk (`status=429 | stats count by user`), then raise the limit via env var.
