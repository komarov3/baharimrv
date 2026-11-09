# Phase 2 Go migration — fix pass resolving 3 prior findings

This commit resolves all three blocking findings from the previous phase 2 review: SMS notifications were delivering UserID (a UUID) to Africa's Talking instead of the fisher's E.164 phone number; Go score tier names were disjoint from the TypeScript mobile client's enum; and FCM push was silently skipped whenever the caller omitted `FCMToken` from `NotifyParams`. All three are now addressed with working code and matching tests. Watch for: the `Retry` method fetches rows with `fcm_status = 'failed'` but only executes SMS retry logic — FCM failures are silently ignored in the retry loop, which is now a real gap since FCM delivery is actually active after this fix.

**Verdict**: APPROVED

---

## High-level view

Finding 1 (SMS phone) is resolved by a `lookupFisher(ctx, userID)` helper that queries `fishers.phone` and `fishers.fcm_token` in a single round-trip whenever SMS or FCM channels are needed. The `Notify` method and `Retry` method both use it. The retry path now correctly re-attempts SMS with the phone number rather than the UUID.

Finding 2 (tier names) is a clean rename throughout the codebase: `tierFromTotal`, the `tierRecommendations` map in `loans/service.go`, and the GORM model default in `models/score.go` all now use `building/developing/established/strong/anchor`, matching the TypeScript `ScoreTier` enum exactly. The test mirror function and all test cases were updated consistently.

Finding 3 (FCM token gap) is resolved by the `lookupFisher` path combined with migration 017, which adds `fcm_token` to `fishers`, and the `PUT /auth/fcm-token` endpoint that lets the mobile app register its token on launch. When no token is registered, `Notify` writes `fcm_status = 'no_token'` rather than leaving the column at its default — observable and queryable.

The `Retry` method's FCM gap is a new concern surfaced by this fix: the query selects rows where `fcm_status = 'failed'`, but the loop body only handles `SMSStatus == "failed"`. Any FCM failure (network error, invalid token, etc.) lands in the retry pool but is never retried. This is confirmed new exposure — before this fix, FCM was never delivered so `fcm_status` was never `'failed'`.

---

<details>
<summary>Issues (1)</summary>

1. **FCM retry dead code** — `Retry` queries for `fcm_status = 'failed'` rows but the loop only branches on `row.SMSStatus == "failed"`. FCM delivery failures accumulate in the retry pool permanently. Fix: add a `row.FCMStatus == "failed"` branch that re-fetches the token via `lookupFisher` and retries `n.fcm.Send`. **confirmed** — `notifications/service.go`: `Retry` queries `fcm_status = 'failed'` at line 131 but the loop body has no FCM branch.

</details>

---

<details>
<summary>Details</summary>

## SMS phone resolution — Finding 1 resolved

`lookupFisher` returns empty strings on error (best-effort), so a missing fisher doesn't block the notification row from being written. Notifications that previously failed with `sms_status = 'failed'` because a UUID was sent to AT will now be retried with the correct phone on the next `Retry` tick.

## Score tier alignment — Finding 2 resolved

`tierFromTotal` cutoffs changed: previously `<40 bronze, <65 silver, <85 gold, >=85 platinum`; now `<40 building, <55 developing, <70 established, <85 strong, >=85 anchor`. The `developing` boundary moves from 65 to 55, meaning fishers in the 55–65 range are now `established` rather than `developing`. This is a scoring behavior change, not just a rename — loan recommendations for those fishers shift from `developing` (KES 15,000 / 4 months / 17%) to `established` (KES 20,000 / 6 months / 15%). The change aligns with the TypeScript tier definitions but the cutoff adjustment should be intentional.

The `loans/service.go` tier map was extended from 4 to 5 entries, adding `developing` with its own product. The fallback for unknown tiers now defaults to `"building"` instead of `"bronze"`.

## FCM token registration path — Finding 3 resolved

Migration 017 uses `ADD COLUMN IF NOT EXISTS`, making it safe to run against a database that already has the column (e.g., re-run after a partial deploy). When no token is registered, `Notify` writes `fcm_status = 'no_token'` — observable in the DB — rather than leaving the column at the empty-string default and silently dropping the push.

## FCM retry gap — new finding

`Retry` queries `WHERE sms_status = 'failed' OR fcm_status = 'failed' OR email_status = 'failed'` and iterates the results. The loop body checks `if row.SMSStatus == "failed"` and retries SMS. There is no `if row.FCMStatus == "failed"` branch. Before this fix, `fcm_status` was never `'failed'` (the goroutine was never launched when `fcmToken == ""`), so this gap had no visible effect. Now that the FCM goroutine runs for users with registered tokens, any `fcm.Send` error sets `fcm_status = "failed"`, and those rows accumulate in the retry pool indefinitely.

</details>

---

<details>
<summary>File map</summary>

| File | What changed |
|---|---|
| `internal/api/auth_handlers.go` | Added `UpdateFCMToken` handler — `PUT /auth/fcm-token` registers device push token |
| `internal/api/router.go` | Wired `PUT /auth/fcm-token` under authenticated routes |
| `internal/loans/service.go` | `tierRecommendations` map extended to 5 tiers; fallback default changed to `"building"` |
| `internal/models/score.go` | `Score` field JSON tag changed from `"score"` to `"total"`; GORM default changed to `"building"` |
| `internal/models/user.go` | `FCMToken string` field added to `Fisher` struct (column `fcm_token`) |
| `internal/notifications/service.go` | `lookupFisher` helper added; `Notify` resolves phone and FCM token in single DB round-trip; SMS sends to phone; `Retry` uses phone lookup; `no_token` status written when FCM requested but token missing |
| `internal/score/service.go` | `tierFromTotal` updated to 5-tier names with revised cutoffs |
| `internal/score/service_test.go` | Test cases and mirror function updated to new tier names |
| `migrations/017_add_fishers_fcm_token.up.sql` | `ALTER TABLE fishers ADD COLUMN IF NOT EXISTS fcm_token` |
| `migrations/017_add_fishers_fcm_token.down.sql` | `DROP COLUMN IF EXISTS fcm_token` |

Full diff: `git -C /Users/Komarov/Documents/Nanga/.worktrees/phase2-domain-services diff HEAD~1...HEAD`

</details>
