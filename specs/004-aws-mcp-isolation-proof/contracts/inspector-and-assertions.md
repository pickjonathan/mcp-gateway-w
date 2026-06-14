# Contract: Inspector Calls + Isolation Assertions

Exact MCP-Inspector invocations and the assertions for each user story. Tokens are minted per realm via
password grant (D6). `BASE` for a tenant = `http://{slug}.mcp.example.com:8080/mcp` (dev hosts entry +
`MCP_RESOURCE_TEMPLATE`).

## Token (per tenant, headless — D6)
```sh
TOKEN=$(curl -s http://localhost:8081/realms/$SLUG/protocol/openid-connect/token \
  -d grant_type=password -d client_id=mcp-client -d scope=openid \
  -d username=$SEED_USER -d password=$SEED_PW | jq -r .access_token)
```

## US1 — functional happy path (FR-007/008; per tenant)
```sh
# list tools
npx @modelcontextprotocol/inspector --cli http://alpha.mcp.example.com:8080/mcp \
  --transport http --header "Authorization: Bearer $ALPHA_TOKEN" --method tools/list
# put an object into the OWN bucket via call_aws
npx @modelcontextprotocol/inspector --cli http://alpha.mcp.example.com:8080/mcp \
  --transport http --header "Authorization: Bearer $ALPHA_TOKEN" \
  --method tools/call --tool-name call_aws \
  --tool-arg cli_command="aws s3 cp /tmp/hello.txt s3://alpha-data/hello.txt"
# get/list back
... --tool-name call_aws --tool-arg cli_command="aws s3 ls s3://alpha-data/"
```
**Assert**: tools listed include `call_aws`; the object round-trips in `alpha-data`; `beta` does the same
against `beta-data`. (Inspector emits JSON to stdout; assert on content, not exit code alone — D5.)

## US2 — adversarial matrix (both ordered pairs; FR-009/010/011)

| # | Invocation (src=alpha, dst=beta shown) | Assert |
|---|---|---|
| V1 | `inspector --cli http://beta…/mcp --header "Authorization: Bearer $ALPHA_TOKEN" --method tools/list` | request rejected (401/403, audience/issuer mismatch); **no beta tool/data** in output; `auth.denied` in `GET /v1/orgs/beta/audit` |
| V2 | `inspector … http://alpha…/mcp --header "…$ALPHA_TOKEN" --method tools/list` | output does **not** contain beta's `aws` server / beta bucket names |
| V3 | `tools/call` targeting beta's server id/slug on alpha's endpoint | method-not-found / forbidden (looks absent); `authz.denied` audited |
| V4 | on alpha's endpoint+token: `call_aws cli_command="aws s3 ls s3://beta-data/"` and `... cp ... s3://beta-data/x"` | **NoSuchBucket / AccessDenied**; beta-data unchanged (account-namespace boundary, preflighted) |
| V6 | scan all V1–V4 stdout/stderr + gateway/server logs | **no** `AWS_SECRET_ACCESS_KEY` value, no bearer token value present |

Repeat with src=beta, dst=alpha.

## SC-010 / V5 — sandbox egress containment (FR-017)
From inside a sandbox on `mcp-sandbox-egress` (probe via the Go live test or a `call_aws`-driven shell):
- `tcp connect ministack:4566` → **success**.
- `tcp connect control-plane:8090`, `vault:8200`, `169.254.169.254:80`, any public host → **fail/timeout**.

## US3 — smoke load + quota independence (FR-012/013; SC-005/006 — D8)
- Driver: `@modelcontextprotocol/sdk` Streamable-HTTP client, **10 concurrent sessions per tenant**, both
  tenants simultaneously, **60 s**, each session looping a small S3 `ls`/`cp` on its **own** bucket.
- Record per tenant: calls, non-quota errors, `error_rate`, `p95_ms`, count of `-32000` (quota) responses.
- **Assert SC-005**: per tenant `error_rate < 1%` (excluding `-32000`), `p95_ms ≤ 2000`.
- **Assert SC-006**: drive alpha past `MCP_RATE_ORG_PER_MIN` → alpha sees `-32000`; **beta's success rate
  unchanged** vs. its pre-contention baseline.
- **Assert SC-004**: continuous leakage scan during the run → **0** cross-tenant artifacts.

## Audit assertion helper (D9)
```sh
curl -s http://localhost:8090/v1/orgs/$DST/audit -H "Authorization: Bearer $DST_ADMIN" \
 | jq -e '.[] | select(.Action=="auth.denied" or .Action=="authz.denied")' >/dev/null
```
Assert a matching record exists with correct `Actor`/`Target` and **no secret** in `Metadata`.
