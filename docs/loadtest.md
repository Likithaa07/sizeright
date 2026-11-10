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
