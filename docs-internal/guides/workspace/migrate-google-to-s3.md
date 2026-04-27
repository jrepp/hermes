---
created: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: dad007fb-34de-4dce-9772-811a51db3acb
status: Draft
title: Migrate a Project from Google Workspace to S3
type: Guide
subtype: Playbook
tags: [workspace, migration, s3, google, operations]
related:
  - ADR-009
  - ADR-017
  - ADR-018
  - RFC-015
---

# Migrate a Project from Google Workspace to S3

> Use this playbook to run a v1.0 copy migration from a Google Workspace provider to an S3-compatible provider. The v1.0 flow is admin-only, preserves the Google provider as canonical, and does not perform rollback or cutover.

## Safety Model

- Only admins can create, inspect, retry, or cancel migration jobs.
- v1.0 supports `strategy: "copy"` only.
- The canonical provider does not change during migration; normal reads and writes continue to use Google unless a request explicitly addresses the S3 provider with a provider-qualified document ID.
- Cancellation stops new item work but does not delete S3 objects already written.
- Lossy fields such as comments, permissions, review state, or revision history must be visible in job and item results when the provider pair cannot preserve them.

## Prerequisites

- Database migrations have already been run with `cmd/hermes-migrate` before the server starts, per [ADR-019](../../adr/adr-019-split-server-and-migrate-binaries.md).
- Google Workspace and S3-compatible providers are configured through HCL and registered in `provider_storage`.
- The S3 bucket exists, versioning is enabled when required, and Hermes can write to the configured prefix.
- You have an admin session or bearer token accepted by the v2 API.
- The operator has chosen an `Idempotency-Key` for each create, retry, or cancel request.

## Example HCL

Use `testing/config-rfc089-migration.hcl` as the current local example for multi-provider storage and S3/MinIO settings. For v1.0 operations, disable automatic migration rules and scheduled tasks; create migrations only through the REST API.

Minimum shape:

```hcl
storage_providers {
  provider "google-prod" {
    type        = "google"
    is_primary  = true
    is_writable = true
    status      = "active"
  }

  provider "s3-archive" {
    type        = "s3"
    is_primary  = false
    is_writable = true
    status      = "active"

    config {
      endpoint            = "http://minio:9000"
      region              = "us-east-1"
      bucket              = "hermes-documents"
      prefix              = "production"
      access_key          = env("HERMES_S3_ACCESS_KEY")
      secret_key          = env("HERMES_S3_SECRET_KEY")
      versioning_enabled  = true
      metadata_store      = "manifest"
      path_template       = "{project}/{uuid}.md"
    }

    capabilities {
      versioning  = true
      permissions = false
      search      = false
      people      = false
      teams       = false
    }
  }
}

providers {
  workspace = "multi"
  search    = "meilisearch"
}
```

## Procedure

1. Confirm the providers are visible and healthy.

```bash
curl -sS -b cookies.txt \
  http://localhost:8001/api/v2/providers
```

2. Create a copy migration job.

```bash
curl -sS -b cookies.txt \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: google-to-s3-hermes-20260427-001' \
  -d '{
    "project_id": "hermes",
    "name": "google-to-s3-rfcs",
    "source_provider": "google-prod",
    "destination_provider": "s3-archive",
    "strategy": "copy",
    "filter": {
      "document_type": "RFC",
      "status": "Published"
    },
    "validate_after_migration": true,
    "concurrency": 5
  }' \
  http://localhost:8001/api/v2/migrations/jobs
```

3. Poll the job until it reaches `completed`, `failed`, or `cancelled`.

```bash
curl -sS -b cookies.txt \
  http://localhost:8001/api/v2/migrations/jobs/0f3e1c9c-1781-43ab-a31e-5b3a6f9b0f15
```

4. Inspect failed or warning items.

```bash
curl -sS -b cookies.txt \
  'http://localhost:8001/api/v2/migrations/jobs/0f3e1c9c-1781-43ab-a31e-5b3a6f9b0f15/items?status=failed&limit=100'
```

5. Retry failed retryable items.

```bash
curl -sS -b cookies.txt \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: google-to-s3-hermes-20260427-retry-001' \
  -d '{"retry_failed_only": true}' \
  http://localhost:8001/api/v2/migrations/jobs/0f3e1c9c-1781-43ab-a31e-5b3a6f9b0f15/retry
```

6. Cancel a pending or running job if the migration must stop.

```bash
curl -sS -b cookies.txt \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: google-to-s3-hermes-20260427-cancel-001' \
  -d '{"reason": "maintenance window ended"}' \
  http://localhost:8001/api/v2/migrations/jobs/0f3e1c9c-1781-43ab-a31e-5b3a6f9b0f15/cancel
```

## Validation Checklist

- Job reports `validation_status: "passed"` or `"passed_with_warnings"`.
- Every completed item has matching source and destination content hashes.
- `docid.UUID`, project, title, document type, and status are unchanged.
- Source and destination provider IDs are recorded for each item.
- Any unpreserved comments, permissions, review state, timestamps, or revision history appear in `lossy_fields`.
- Normal document reads still route to Google unless the request explicitly names the S3 provider.

## Failure Handling

| Symptom | Action |
|---|---|
| Provider missing from `GET /api/v2/providers` | Fix HCL or provider registration, rerun migrations if schema rows are missing, then restart Hermes. |
| `403` on every endpoint | Confirm the session belongs to an admin user. v1.0 does not allow project-owner migration operations. |
| `409` on job creation | Reuse the same request body for the same `Idempotency-Key`, or choose a new key for a deliberately different job. |
| Retry returns `409` | Inspect item states; completed, in-progress, skipped, and non-retryable failed items cannot be retried. |
| Cancellation leaves S3 objects behind | Expected for v1.0. Delete or archive destination objects manually only after reviewing item results. |
| Metadata warnings after completion | Treat `passed_with_warnings` as a successful content migration with provider-specific loss. Decide separately whether those fields block cutover. |

## Do Not Do This in v1.0

- Do not edit `migration_jobs`, `migration_items`, or `provider_storage` rows by hand except under incident response.
- Do not treat copy completion as cutover; canonical routing remains unchanged.
- Do not enable automatic or recurring migration rules for this flow.
- Do not expect rollback to remove destination objects.
