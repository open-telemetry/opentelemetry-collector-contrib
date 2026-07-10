# PostgreSQL Blocking Session Detection — What to Capture and Why

## The Core Problem

When a PostgreSQL session is blocked, there are two sides:

- **Blocked session** — waiting to acquire a lock it cannot get yet
- **Blocker(s)** — one or more sessions holding conflicting locks on the same resource

The key insight: **a session can be blocked by multiple sessions simultaneously**, and it must wait for ALL of them to release before it can proceed.

---

## Concrete Example

```
Table: orders
```

| Session | Operation | Lock held/requested | State |
|---|---|---|---|
| 21 | `SELECT * FROM orders` | `AccessShareLock` on orders | Active, holding |
| 22 | `SELECT * FROM orders FOR UPDATE` | `RowShareLock` on orders | Active, holding |
| 33 | `UPDATE orders SET ...` (not committed) | `RowExclusiveLock` on orders | Idle in transaction, holding |
| **10** | `ALTER TABLE orders ADD COLUMN ...` | `AccessExclusiveLock` on orders | **Waiting** |

Session 10 needs `AccessExclusiveLock`, which conflicts with ALL three modes held by 21, 22, and 33.

`pg_blocking_pids(10) = {21, 22, 33}` — session 10 is blocked by all three **simultaneously**. It cannot proceed until the **last one** releases. This is not a queue — it is a true AND condition.

---

## What `pg_locks` looks like at this moment

```
pid | granted | locktype | mode                 | relation
----|---------|----------|----------------------|----------
21  | TRUE    | relation | AccessShareLock      | orders   ← blocker, holding
22  | TRUE    | relation | RowShareLock         | orders   ← blocker, holding
33  | TRUE    | relation | RowExclusiveLock     | orders   ← blocker, holding (idle in txn)
10  | FALSE   | relation | AccessExclusiveLock  | orders   ← blocked, waiting
```

---

## What our query captures today

```sql
LEFT JOIN pg_locks bl
  ON  bl.pid = sa.pid
  AND NOT bl.granted        -- the blocked session's OWN waiting lock row
LEFT JOIN pg_class c
  ON  c.oid = bl.relation
```

For session 10's row we get:

```
pid              = 10
blocking_pids    = {21, 22, 33}      -- all blockers
lock_mode        = AccessExclusiveLock   -- what session 10 is REQUESTING
lock_type        = relation              -- type of resource being waited on
locked_relation  = orders               -- the actual table name
blocking_start_time = 2025-01-15T10:23:45Z
```

**What this tells us:**
- Session 10 is waiting to acquire `AccessExclusiveLock` on `orders`
- It is blocked by 3 sessions simultaneously
- It has been blocked since `blocking_start_time`
- The severity is high — `AccessExclusiveLock` is the most exclusive lock (schema changes, TRUNCATE)

**What this does NOT tell us:**
- What lock mode each blocker (21, 22, 33) is holding
- Whether session 33 is `idle in transaction` (a common dangerous pattern)
- How long each blocker has been holding their lock

---

## Can `lock_type` differ across blockers?

**No.** All blockers in `{21, 22, 33}` are conflicting on the same lock request from session 10. They all hold locks on the same resource. The `lock_type` (relation, transactionid, tuple, etc.) describes the resource — it is the same for all blockers by definition.

```
Session 10 waiting: locktype=relation on orders
  Session 21 holds: locktype=relation on orders   ← same type
  Session 22 holds: locktype=relation on orders   ← same type
  Session 33 holds: locktype=relation on orders   ← same type
```

You cannot have session 10 waiting on `locktype=relation` while being blocked by a session holding `locktype=transactionid` — those are different resources.

---

## Can `lock_mode` differ across blockers?

**Yes.** Each blocker acquired their lock independently for their own operation. Different operations take different modes, all of which may conflict with what session 10 needs.

```
Session 10 wants: AccessExclusiveLock
  Session 21 holds: AccessShareLock      (plain SELECT)
  Session 22 holds: RowShareLock         (SELECT FOR SHARE)
  Session 33 holds: RowExclusiveLock     (UPDATE, uncommitted)
```

All three conflict. All three must release. Three different modes.

---

## The "multiple locks per blocker" problem

Each blocker can hold **many** granted locks across many tables. If we tried to capture the blockers' held locks:

```sql
LEFT JOIN pg_locks held
  ON held.pid = ANY(pg_blocking_pids(sa.pid))
  AND held.granted = TRUE
```

Session 21 alone might have 15 granted locks (on `orders`, `customers`, `products`, internal PostgreSQL system tables, etc.). With 3 blockers you'd get 30–50 rows for a single blocked session. The join explodes.

db-agent (AppDynamics) avoids this entirely — they do not capture blocker lock modes at all.

---

## What could we additionally capture (and tradeoffs)

### Option 1: Blocker session state (low cost, high value)

Add `blocker_act.state` from a second join back to `pg_stat_activity` for each blocker pid:

```sql
LEFT JOIN pg_stat_activity blocker_act
  ON blocker_act.pid = ANY(pg_blocking_pids(sa.pid))
```

**Problem:** Same row explosion — if `{21,22,33}` then 3 extra rows per blocked session.

**Alternative:** Use a lateral subquery to capture the most dangerous blocker state:
```sql
LEFT JOIN LATERAL (
  SELECT bool_or(state = 'idle in transaction') AS has_idle_in_txn_blocker,
         count(*) AS blocker_count
  FROM pg_stat_activity
  WHERE pid = ANY(pg_blocking_pids(sa.pid))
) bl_meta ON TRUE
```

**Value:** Knowing a blocker is `idle in transaction` (forgot to commit) is the #1 actionable signal for DBAs.

### Option 2: Blocker lock mode per pid (high cost, moderate value)

Build a JSON array of `{pid, mode}` per blocker:

```sql
LEFT JOIN LATERAL (
  SELECT json_agg(json_build_object('pid', bl.pid, 'mode', lk.mode))::text AS blockers_with_mode
  FROM unnest(pg_blocking_pids(sa.pid)) bl(pid)
  LEFT JOIN pg_locks lk ON lk.pid = bl.pid AND lk.granted AND lk.relation = (
    SELECT relation FROM pg_locks WHERE pid = sa.pid AND NOT granted LIMIT 1
  )
) bl_modes ON TRUE
```

**Value:** Explains exactly why each blocker conflicts.
**Cost:** Extra subquery per row, complex, fragile.

### Option 3: Root blocker (chain walking)

For blocked chains (10 blocked by 21, 21 blocked by 22), find the root:

```sql
-- Not natively possible in a single query without recursive CTE
-- pg_blocking_pids only gives immediate blockers
```

Oracle has `V$SESSION.FINAL_BLOCKING_SESSION` for free. PostgreSQL requires a recursive CTE to walk the chain and find the true root holder.

**Value:** High — tells you who to kill to unblock the whole chain.
**Cost:** Recursive CTE, multiple self-joins, expensive on large `pg_stat_activity`.

---

## What we should capture (recommended set)

| Column | Source | Why |
|---|---|---|
| `blocking_pids` | `pg_blocking_pids(sa.pid)` | All immediate blockers |
| `lock_mode` | blocked session's `pg_locks.mode` (NOT granted) | Severity of contention |
| `lock_type` | blocked session's `pg_locks.locktype` | Resource type |
| `locked_relation` | `pg_class.relname` via `pg_locks.relation` | Human-readable object name |
| `blocking_start_time` | `sa.state_change` when blockers exist | How long blocked |
| `transaction_start` | `sa.xact_start` | How long the blocked txn has been open |

**Not captured (acceptable tradeoff):**
- Blocker lock modes — requires row explosion or complex lateral
- Root blocker — requires recursive CTE, not worth the cost at sample frequency
- Blocker session state — useful but requires additional join; can derive from a second query

**The idle blocker gap (critical):**

```sql
OR sa.pid IN (
  SELECT unnest(pg_blocking_pids(pid))
  FROM   pg_stat_activity
  WHERE  cardinality(pg_blocking_pids(pid)) > 0
)
```

This clause captures sessions that are `idle in transaction` but still holding locks. This is the most dangerous real-world scenario — a developer opened a transaction, ran a query, then walked away. The session is INACTIVE but blocking everyone. **db-agent misses this entirely** because they only query `WHERE state = 'active'`.
