# Contract: Per-Tenant AWS Setup (reuses existing APIs)

How the harness builds each tenant's isolated AWS workload. **No new API** — all calls are existing
`003`/control-plane endpoints. Shown for `alpha`; `beta` is identical with account `222222222222`.

## 1. Provision tenant (platform-admin token — `003`)

```http
POST http://localhost:8090/v1/platform/tenants
Authorization: Bearer <platform-operator token from _platform realm>
{ "slug": "alpha", "display_name": "Alpha", "admin_email": "admin@alpha.example" }
→ 202 { "tenant": { "slug": "alpha", "realm_name": "alpha", "status": "provisioning", ... },
        "job": { "id": "...", "status": "pending", ... } }
```
Harness polls `GET /v1/platform/tenants/alpha` until `status=active` and `subdomain_ready=true`.

## 2. Register the AWS stdio server (alpha org-admin token)

```http
POST http://localhost:8090/v1/orgs/alpha/servers
Authorization: Bearer <alpha admin token>
{
  "slug": "aws",
  "type": "stdio",
  "command": "aws-api-mcp-server",
  "args": [],
  "env": { "AWS_ENDPOINT_URL": "http://ministack:4566",
           "AWS_REGION": "us-east-1",
           "AWS_API_MCP_WORKING_DIR": "/tmp" },
  "credential_mode": "org",
  "allowed_roles": []
}
→ 201 { "id": "<server-id>", "slug": "aws", "type": "stdio", "credential_set": false, ... }
```
> `command` is the pre-baked entrypoint from `deploy/sandbox-images/Dockerfile` (D2). `env` holds only
> **non-secret** config. `READ_OPERATIONS_ONLY` is left unset (writes needed for `s3 mb`/`cp`).

## 3. Set write-only AWS credentials (alpha org-admin token — FR-006)

```http
PUT http://localhost:8090/v1/orgs/alpha/servers/<server-id>/credentials
Authorization: Bearer <alpha admin token>
{ "AWS_ACCESS_KEY_ID": "111111111111", "AWS_SECRET_ACCESS_KEY": "<random-alpha-secret>" }
→ 204 No Content        # values are NEVER returned; only `credential_set` flips true
```
At launch these are injected as env into the sandboxed server (`server/creds.go` `kvEnv`), alongside the
non-secret `env` from step 2. Account id = the 12-digit access key (D1).

## 4. Create the tenant's bucket (under its own account)

Either path (both use **only** alpha's account creds):
- **Through the server** (preferred, exercises the full path): Inspector `tools/call call_aws` with
  `{"cli_command":"aws s3 mb s3://alpha-data"}`.
- **Out-of-band** (setup convenience): harness S3 client to `http://localhost:4566` with alpha creds →
  `createBucket(alpha-data)`.

## Resulting per-tenant isolation inputs

| Tenant | Realm/issuer | MCP audience (dev) | AWS account | Secret | Bucket |
|---|---|---|---|---|---|
| alpha | `…/realms/alpha` | `http://alpha.mcp.example.com:8080/mcp` | `111111111111` | write-only | `alpha-data` |
| beta  | `…/realms/beta`  | `http://beta.mcp.example.com:8080/mcp`  | `222222222222` | write-only | `beta-data` |

## Teardown (SC-008)
- `DELETE /v1/orgs/{slug}/servers/{id}/credentials` then `DELETE …/servers/{id}`.
- `DELETE /v1/platform/tenants/{slug}` (purges realm/identity per `003`).
- Delete buckets `alpha-data`/`beta-data` from the emulator.
- Post-check: re-list servers/tenants/buckets → none of the feature-created resources remain.
