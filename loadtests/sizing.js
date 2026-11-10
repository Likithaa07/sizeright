import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 50 },
    { duration: '30s', target: 100 },
    { duration: '30s', target: 200 },
    { duration: '30s', target: 400 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<200'],
  },
};

const regions = ['ap-southeast-2', 'us-east-1', 'us-west-2', 'eu-west-1'];

export default function () {
  const body = JSON.stringify({
    customer_id: `load${Math.floor(Math.random() * 1000)}`,
    cloud: 'aws',
    region: regions[Math.floor(Math.random() * regions.length)],
    ingest_gb_per_day: (Math.floor(Math.random() * 50) + 1) * 100,
    retention_days: 90,
    search_profile: 'moderate',
    concurrent_users: 40,
    premium_apps: [],
    smartstore: true,
    high_availability: true,
  });
  const res = http.post('http://localhost:8080/v1/sizings', body, {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${__ENV.TOKEN}` },
  });
  check(res, { 'status is 201': (r) => r.status === 201 });
}
