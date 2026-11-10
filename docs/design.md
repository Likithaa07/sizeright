# SizeRight: Design

## Problem
Splunk Cloud customers buy a subscription (data per day, retention, users, add-ons).
Someone must decide how many servers and how much storage each customer needs.
SizeRight automates that decision.

## Users
- Sizing engineers: create sizings
- Viewers (support, sales): read sizings
- Provisioning automation: downloads Terraform inputs

## Inputs
customer_id, cloud, region, ingest_gb_per_day, retention_days, search_profile,
concurrent_users, premium_apps, smartstore, high_availability

## Outputs
indexers, core_search_heads, premium_search_heads, local_storage_tb,
object_store_tb, explanations, rules_version

## API
- GET  /healthz                      : is the app alive
- POST /v1/sizings                   : create a sizing
- GET  /v1/sizings/{id}              : fetch a sizing
- GET  /v1/sizings/{id}/terraform    : Terraform inputs

## Sizing rules
Kept in rules/v1.yaml (versioned). Based on public Splunk sizing guidance:
~100 GB/day per indexer with Enterprise Security, 150-300 otherwise;
min 3 indexers for HA; storage = ingest x (RF x 0.15 + SF x 0.35).

## Open questions
- How should ITSI change the sizing?
- Should rules differ per cloud provider?
