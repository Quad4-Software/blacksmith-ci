<!-- markdownlint-disable MD041 -->

# Cross-forge account takeover via unscoped user lookup on login

**Severity:** Critical
**CVSS v3.1:** 9.8 (`AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H`)
**CWE:** CWE-287 (Improper Authentication), CWE-290 (Authentication Bypass by Spoofing)
**CVE:** requested from MITRE, not yet assigned. This advisory is updated when the ID is
issued.
**Affected:** Crow 5.2.0 through 6.3.0
**Fixed in:** v6.4.0
**Reported by:** Cassandra Beelen ([sapphiccode](https://sapphiccode.net/))

If you run Crow with more than one forge configured, upgrade to v6.4.0 and then work
through [Detecting prior exploitation](#detecting-prior-exploitation) — the fix stops new
takeovers but does not undo one that already succeeded.

## Summary

When a user signed in through a forge, Crow resolved the matching Crow account with a
lookup that was not scoped to the forge the login came from.
The lookup matched on `users.forge_remote_id` alone and, failing that, on `users.login`
alone.
Neither value is unique across forges, so an account on one forge could resolve to a
user registered on a different forge and log in as them.

On an instance with more than one forge configured, anyone able to register on any
connected forge could take over an arbitrary Crow account, including administrators, by
claiming the target's username there.

## Impact

A successful attacker obtains a full session for the victim's Crow account.
Where the victim is an administrator, that includes:

- reading every repository, organization and global secret and registry credential
- modifying or triggering pipelines on any repository
- managing users, agents and instance settings

`CROW_OPEN=false` did not prevent this.
The registration-closed check only guards the branch that creates a _new_ user, and a
takeover login never reached it: the lookup returned an existing user, so Crow treated
the request as a normal sign-in.

## Affected configurations

- **Instances with two or more forges configured: affected.**
  An attacker registers the victim's Crow username on any connected forge and signs in.
  A username match is not even required in every case: forge remote IDs are small
  per-forge sequences, so an attacker whose numeric user ID on forge B happens to equal
  the victim's stored `forge_remote_id` matches on the first lookup regardless of name.
- **Single-forge instances: affected in a narrower case.**
  If a user deletes their forge account and someone else registers the same username on
  that forge, the new account receives a fresh remote ID, the remote-ID lookup misses,
  and the login-name fallback resolves to the original Crow account.

## Fix

`GetUserRemoteID(remoteID, login)` is replaced by
`GetUserByForgeRemoteID(forgeID, remoteID, login)`, which scopes both the remote-ID
lookup and the login-name fallback to the forge that authenticated the request.
The login-name fallback is further restricted to rows that have no `forge_remote_id`,
which is the only case it was ever meant to serve (accounts predating remote-ID
tracking).
Rows with an unset `forge_id` continue to resolve, as belonging to the main forge.

Registration additionally refuses a username already held by a user from another forge.
`users.login` is unique instance-wide, so this now fails closed with a clear error
rather than a unique-constraint violation surfaced as an internal error.

## Detecting prior exploitation

The fix prevents new takeovers but does not undo one that already happened: a forge link
created through the vulnerable path is indistinguishable from a legitimate one at
sign-in time.
It is, however, visible in the database.
A forge linked the intended way (Settings > Linked Forges) is stored with
`is_primary = false`, whereas the vulnerable path stored `is_primary = true`.
Any account with more than one primary forge link therefore went through it:

```sql
SELECT u.id, u.login, u.admin, COUNT(*) AS primary_links
FROM user_forges uf
JOIN users u ON u.id = uf.user_id
WHERE uf.is_primary
GROUP BY u.id, u.login, u.admin
HAVING COUNT(*) > 1;
```

To inspect the individual links for a flagged account:

```sql
SELECT forge_id, forge_login, forge_email, is_primary, linked_at
FROM user_forges
WHERE user_id = <id>
ORDER BY linked_at;
```

A hit is not proof of an attack.
The same rows are produced when one person legitimately owns matching accounts on both
forges and signed in through each.
Review the flagged accounts, and treat a link you cannot account for as a compromise.

Not every takeover leaves this trace.
If the attacker signed in before the victim ever did, the attacker's link is the account's
only primary row.
On an instance where the finding matters, audit `linked_at` against your own records
rather than relying on the query alone.

## Response for a compromised instance

1. Upgrade to v6.4.0.
2. Run the detection query and review every flagged account.
3. Delete forge links you cannot account for, from user settings or directly in
   `user_forges`.
4. Invalidate the sessions of affected users.
   Sessions and API tokens are signed with the per-user `users.hash`, so resetting that
   value logs out every session for the user; the "reset token" action in user settings
   does this, or rotate the column directly.
5. Rotate every secret and registry credential reachable by a flagged account.
6. Review pipeline history on repositories the flagged accounts could reach, for
   pipelines triggered or modified during the exposure window.

## Credit

Found and reported privately by Cassandra Beelen
([sapphiccode](https://sapphiccode.net/)), with a clear reproduction and a 180-day
coordinated-disclosure window.
The fix shipped well inside it.
Thank you.

## Timeline

| Date       | Event                                 |
| ---------- | ------------------------------------- |
| 2026-08-05 | Reported privately to the maintainers |
| 2026-08-10 | Root-caused, fixed and tested         |
| 2026-08-10 | v6.4.0 released with this advisory    |
