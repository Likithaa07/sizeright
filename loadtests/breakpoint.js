import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 500 },
    { duration: '30s', target: 1000 },
    { duration: '30s', target: 2000 },
    { duration: '30s', target: 3000 },
    { duration: '30s', target: 4000 },
  ],
  thresholds: {
    http_req_failed: [{ threshold: 'rate<0.01', abortOnFail: true, delayAbortEval: '10s' }],
  },
};

const regions = ['ap-southeast-2', 'us-east-1', 'us-west-2', 'eu-west-1'];

export default function () {
  const body = JSON.stringify({
    customer_id: `load${Math.floor(Math.random() * 1000)}`,
    cloud: 'aws',
    region: regions[Math.floor(Math.random() * regions.length)],
    ingest_gb_per_day: (Math.floor(Math.random() * 50) + 1) * 100,
    retention_days: 90, search_profile: 'moderate', concurrent_users: 40,
    premium_apps: [], smartstore: true, high_availability: true,
  });
  const res = http.post('http://localhost:8080/v1/sizings', body, {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${__ENV.TOKEN}` },
    timeout: '10s',
  });
  check(res, { 'status is 201': (r) => r.status === 201 });
}
