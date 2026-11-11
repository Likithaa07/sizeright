# Load test results (k6)

Setup: MacBook Air M4, API + Postgres + Redis on the same machine (local benchmark).
Script: `loadtests/sizing.js` — POST /v1/sizings, ramp 50 → 100 → 200 → 400 virtual users over 2m15s.
Rate limit raised for the test (`RATE_LIMIT_PER_MIN=1000000`).

| Metric | Result | Target |
|---|---|---|
| Total requests | 854,789 | — |
| Throughput | ~6,330 req/s | — |
| Error rate | 0.00% | < 1% |
| p95 latency | 56.1 ms | < 200 ms |
| p90 latency | 50.6 ms | — |
| Median latency | 16.0 ms | — |
| Max latency | 968 ms | — |
| Peak concurrent users | 400 | — |

Result: both thresholds passed; no breaking point reached at 400 VUs.
Next: run k6 from a separate machine against the deployed API.

## Breaking point + fix (2026-11-11)

Script: `loadtests/breakpoint.js` ramps to 4,000 virtual users and aborts if >1% of requests fail.

- **It never crashed:** 1,040,199 requests, 0% errors at up to 4,000 users.
- **It saturated:** throughput flattened at ~6,900 req/s and p95 latency rose from 56 ms (400 users) to 588 ms (4,000 users).
- **Root cause:** the Postgres driver's default pool is ~10 connections (one per CPU core), so requests queued for a connection.
- **Fix:** `pool_max_conns=50` in `DATABASE_URL`.

Same test before/after (2,000 users, 60 s):

| | Pool 10 (default) | Pool 50 | Change |
|---|---|---|---|
| Throughput | 7,276 req/s | 10,284 req/s | **+41%** |
| Avg latency | 274 ms | 194 ms | −29% |
| p95 latency | 314 ms | 272 ms | −13% |
| Errors | 0% | 0% | — |

Remaining limit: k6, the API and Postgres share one laptop CPU. Next: run k6 from another machine.
