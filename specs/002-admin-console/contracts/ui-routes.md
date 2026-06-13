# Contract — UI routes & screens

The console's client-side routes, each screen's data dependencies, the access it
requires, and the user story it serves. All routes are within the signed-in org;
all require the **admin** role except sign-in/callback/forbidden.

| Route | Screen | Data (endpoints) | Access | Story |
|---|---|---|---|---|
| `/signin` | Sign-in (redirect to realm) | — (OAuth PKCE) | public | US1 |
| `/callback` | OAuth redirect handler | token exchange | public | US1 |
| `/forbidden` | Access denied | — | authenticated, non-admin | US1 |
| `/` | Dashboard | `GET …/servers`, `GET …/audit` | admin | US1, US6 |
| `/servers` | Servers catalog (table) | `GET …/servers` | admin | US2, US4 |
| `/servers/new` | Add server (remote \| stdio form) | `POST …/servers` | admin | US2 |
| `/servers/{id}` | Server detail (Tabs: Overview / Credentials / Access / Health) | `GET …/servers/{id}`, `PATCH`, `PUT/DELETE …/credentials[/me]` | admin | US2, US3, US4 |
| `/servers/{id}/edit` | Edit server | `PATCH …/servers/{id}` | admin | US2 |
| `/audit` | Audit log (table + filters + chain banner) | `GET …/audit` | admin | US5 |
| `/settings` | Org settings: rate limits + connection endpoint | quotas (see gap) + derived endpoint | admin | US6 |

## Screen → Carbon composition (adherence, FR-021)

- **Shell**: cloud-console UI-kit shell (header + side nav + content) from the
  handoff; org/brand in the header; profile/sign-out menu.
- **Dashboard**: Tile metric cards; Tag for health/status; ProgressBar for usage;
  recent-activity list.
- **Servers / Audit tables**: Search + Select filters; Tag status; Tabs on detail;
  Button row actions; pagination.
- **Forms** (add/edit, credentials, settings): TextInput, Select (type/mode),
  Toggle (enabled), Checkbox (roles); InlineNotification for success/error;
  destructive actions use a confirmation with danger Button.
- **States**: every data view has loading (skeleton), empty (guided CTA), and
  error (InlineNotification) states (FR-020).

## Cross-cutting guards (Constitution I, VI)

- Route guard: unauthenticated → `/signin`; authenticated non-admin → `/forbidden`.
- Every API call carries the org-scoped bearer; the console never constructs a
  request for an org other than the session's.
- No screen, tooltip, copy action, or error ever renders a stored secret value
  (FR-013) — enforced by an adversarial test.
