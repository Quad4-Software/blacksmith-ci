# Multi-Scope Secrets

Allow a single secret to be shared with multiple repos, orgs, or users (personal orgs) instead of duplicating it across entities.

## Problem

Secrets can currently be scoped to exactly one repo, one org, or globally.
Users who need the same credential (e.g., a deploy key or registry token) across multiple repos must create duplicate secrets manually.

## Core Concepts

### First Owner

Every secret has a **first owner** — the entity (repo, org, or global) under which it was originally created.
The first owner is represented by the existing `org_id`/`repo_id` fields on the `secrets` table.
Ownership can be transferred to any other entity type; the old owner can optionally retain access as a shared target.

### Shared Targets

A secret can be additionally shared with other repos and orgs via a junction table.
"User scope" maps to the user's personal org — no new entity type.

### Priority & Conflict Rules

1. **First-owner secret always wins** over any shared secret with the same name (silent override).
2. **Own + shared conflict** with the same name is visually highlighted in the UI as a warning (not an error — own wins).
3. **Multiple shared secrets** with the same name reaching the same entity are **rejected at assignment time** (error on `SecretTargetCreate`), with a runtime safety-net check in pipeline resolution.

### Pipeline Resolution Priority Ladder

1. First-owner repo secret
2. First-owner org secret
3. First-owner global secret
4. Shared secrets (via `secret_targets`)

Deduplication by name; highest priority wins.

## Database Schema

### No changes to `secrets` table

The existing `org_id`/`repo_id` fields continue to represent the first owner.

### New `secret_targets` table

```sql
CREATE TABLE secret_targets (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    secret_id  INTEGER NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
    org_id     INTEGER NOT NULL DEFAULT 0,
    repo_id    INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER,
    UNIQUE(secret_id, org_id, repo_id)
);
```

- `org_id=X, repo_id=0` — shared with org X (and all its repos)
- `org_id=0, repo_id=Y` — shared with repo Y specifically
- `ON DELETE CASCADE` cleans up targets when a secret is deleted

### Conflict validation at insert time

Before inserting into `secret_targets`, check that no other secret with the same name already targets the same entity (either as first owner or via another `secret_targets` row).
Reject with an error if a conflict would occur.

### First owner transfer

Update the secret's `org_id`/`repo_id` to the new owner.
The old owner can optionally become a target row to retain access.

## Store Layer Changes

### New model

```go
type SecretTarget struct {
    ID        int64
    SecretID  int64
    OrgID     int64
    RepoID    int64
    CreatedAt int64
}
```

### New methods on `SecretStore`

```go
SecretTargetCreate(secretID int64, orgID int64, repoID int64) error
SecretTargetDelete(secretID int64, orgID int64, repoID int64) error
SecretTargetList(secretID int64) ([]*SecretTarget, error)
SecretTransferOwner(secretID int64, newOrgID int64, newRepoID int64, keepOldOwnerAsTarget bool) error
```

### Modified query logic

- `SecretList(repo, ...)` — also returns secrets where `secret_targets` has a matching `repo_id = repo.ID` OR `org_id = repo.OrgID`
- `OrgSecretList(orgID, ...)` — also returns secrets shared with that org via `secret_targets`
- `SecretListPipeline(repo, pipeline)` — includes shared secrets, applies priority ladder, errors on multiple shared secrets with same name
- `UserSecretList(...)` — includes secrets shared with the user's orgs/repos

Results include a way to distinguish "first owner" from "shared" (e.g., `is_shared` field on the returned struct).

## API Changes

### New endpoints for target management

```text
POST   /secrets/{secret_id}/targets              — add a target (body: {org_id, repo_id})
DELETE /secrets/{secret_id}/targets/{target_id}   — remove a target
GET    /secrets/{secret_id}/targets               — list all targets for a secret
PATCH  /secrets/{secret_id}/owner                 — transfer first owner (body: {org_id, repo_id, keep_as_target: bool})
```

### Authorization

- Only the first owner's admins can add/remove targets or transfer ownership.
- Repo-owned secrets: requires push access to the repo.
- Org-owned secrets: requires org admin.
- Global secrets: requires instance admin.

### Response changes to existing list endpoints

- `is_shared` boolean — whether the secret is shared into the current scope (not first-owned)
- `first_owner_name` string — where a shared secret originates
- Shadowing info when a first-owner secret overrides a shared secret with the same name

## Frontend Changes

### Secret list views (repo/org/user)

- Shared secrets show a "shared" badge with the first owner's name.
- Shadowed secrets (same name, lower priority) get a warning icon/tooltip explaining they are overridden.

### Target management UI

- On the secret detail/edit view, a "Shared with" section lists current targets.
- Each target renders as a chip/tag with the org/repo name and a remove button.
- "Add target" button opens a search/select dialog to pick repos or orgs to share with.
- Error shown inline if adding a target would create a name conflict.

### Ownership transfer

- In the "Shared with" section, each target has a "Make owner" action.
- Confirmation dialog explaining the current owner becomes a target (or loses access).
- The current first owner is visually distinguished ("Owner" label vs "Shared" label).

## Testing Strategy

### Store layer tests (most critical)

- CRUD for `secret_targets` (create, delete, list)
- Conflict detection: reject target creation when same-name secret already targets that entity
- Conflict detection: reject target creation when first-owner secret on that entity has the same name
- `SecretList` returns both first-owner and shared secrets for a repo/org
- `SecretListPipeline` priority ordering: first-owner beats shared, same-tier dedup works
- `SecretListPipeline` runtime error on duplicate shared secret names (safety net)
- Ownership transfer: fields update correctly, old owner optionally becomes target
- Ownership transfer: reject if no new owner specified
- CASCADE delete: removing a secret removes all its targets
- `UserSecretList` includes shared secrets
- All tests run against SQLite, Postgres, and MySQL

### API handler tests

- Target CRUD endpoints: happy path, authorization, validation errors
- Ownership transfer endpoint: happy path, authorization, keep-as-target flag
- Existing secret list endpoints return shared secrets with correct `is_shared` and `first_owner_name`
- Authorization: only first owner's admins can manage targets

### Service layer tests

- Pipeline resolution with mixed first-owner and shared secrets
- Shadowing behavior (first-owner wins silently)
- Error on multiple shared secrets with same name

### Frontend tests

- Shared badge renders for shared secrets
- Shadow warning renders when applicable
- Target management: add/remove targets, ownership transfer confirmation dialog
- Conflict error displayed inline
