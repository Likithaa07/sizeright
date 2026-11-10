# Postmortem: Redis outage caused sizing requests to fail

**Date:** 2026-11-10 · **Severity:** high (all new sizings failed) · **Found by:** chaos drill · **Status:** fixed

## Summary
When Redis was stopped, every POST /v1/sizings returned 500 after ~2s, even though the cache was designed to "fail open".

## Impact
100% of sizing requests failed while Redis was down (drill only, no real users).

## Timeline (AEDT)
- 23:24:59 Redis stopped as part of Drill 2.
- 23:25:01 First 500: `insert sizing: context deadline exceeded`, `duration_ms: 2001`.
- 23:26 Root cause found from logs.
- 23:28 Fix + regression test written; test passes.
- 23:29 Drill re-run: 3/3 requests return 201 with Redis down.

## Root cause
The cache shared the request's 2-second timeout. With Redis down, the Redis client retried the connection 5 times per call (`failed to dial after 5 attempts`). Get + Set retries used up the whole 2s budget, so the Postgres insert that followed had no time left and failed with `context deadline exceeded`.

## What went well
- Structured logs with `request_id` and `duration_ms` made the cause obvious within minutes.
- Health checks behaved correctly.

## Fix
- Each cache call gets its own 100 ms budget (`context.WithTimeout`).
- Redis client retries disabled (`MaxRetries: -1`).
- Regression test `TestRedisDownFailsFast`: cache calls must finish in < 300 ms when Redis is unreachable.

## Lessons
- "Fail open" is not enough. An optional dependency must also **fail fast**, with its own time budget.
- Run chaos drills on every dependency, including the "harmless" ones.

## Follow-ups
- [ ] Add a circuit breaker so the app skips Redis entirely for 30s after repeated failures.
- [ ] Add a Splunk alert on `redis: connection pool` log lines.
