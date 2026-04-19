# Phase 3 Build Log

Running log of the Phase 3 implementation. One entry per commit (or notable decision). Paired with `Phase3_Implementation.md` — that doc is the plan; this is what actually happened.

---

## 2026-04-19 — kickoff

Branched off `phase-3` (tip `9be7593`, design docs). Starting from the commit plan in `Phase3_Implementation.md`. Aiming to land commits 1–3 (groundwork + package skeleton) and then pause for review before the DAG lands.

Repo state check before starting:
- `server/internal/db/db.go` migrates `players` only — no `ships` table yet, matching the Impl doc's "starting state" section.
- `handlers/player.go` returns `{id, seed}` only — no ship_id, no ship insert.
- No `internal/factory` package yet.

So commit 1 is indeed the missing Phase 2 groundwork, exactly as the Impl doc warns.
