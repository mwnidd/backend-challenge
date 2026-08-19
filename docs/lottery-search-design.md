# Lottery Search System Design

## Problem

Support search over 10 million lottery tickets where each ticket is a 6-digit number. Queries are 6-character patterns containing digits and `*` wildcards, for example `****23`, `1****5`, and `123***`.

The system must also prevent the same search pattern from assigning the same ticket to multiple users at the same time.

## Recommended Storage

Use PostgreSQL for the primary production implementation.

Reasons:

- The domain is small and fixed: every ticket has only 6 positions.
- PostgreSQL handles transactional allocation and row-level locks well.
- B-tree indexes on generated digit-position columns support efficient wildcard filtering.
- Operational complexity is lower than introducing a search engine for this exact matching problem.
- `SELECT ... FOR UPDATE SKIP LOCKED` is a good fit for concurrent reservation without duplicate assignment.

Redis can be added as a short-lived cache/reservation accelerator for very high request bursts, but PostgreSQL should remain the source of truth for ticket state.

## Data Model

Store tickets in a table:

```sql
CREATE TABLE lottery_tickets (
  id BIGSERIAL PRIMARY KEY,
  ticket_number CHAR(6) NOT NULL,
  d1 CHAR(1) NOT NULL,
  d2 CHAR(1) NOT NULL,
  d3 CHAR(1) NOT NULL,
  d4 CHAR(1) NOT NULL,
  d5 CHAR(1) NOT NULL,
  d6 CHAR(1) NOT NULL,
  status TEXT NOT NULL DEFAULT 'available',
  reserved_by TEXT,
  reserved_for_pattern CHAR(6),
  reserved_until TIMESTAMPTZ,
  allocated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Keep one generated/stored column per digit position. For example, ticket `123456` has `d1='1'`, `d2='2'`, and so on.

## Indexing Strategy

There are only 64 wildcard masks for a 6-character pattern. For each non-empty mask, create a partial composite index matching the specified positions.

Examples:

```sql
CREATE INDEX idx_lottery_d5_d6_available
  ON lottery_tickets (d5, d6, id)
  WHERE status = 'available';

CREATE INDEX idx_lottery_d1_d6_available
  ON lottery_tickets (d1, d6, id)
  WHERE status = 'available';

CREATE INDEX idx_lottery_d1_d2_d3_available
  ON lottery_tickets (d1, d2, d3, id)
  WHERE status = 'available';
```

For complete coverage, generate indexes for all useful digit-position combinations. Because the ticket length is fixed at 6, the index count is bounded and manageable.

Alternative for reduced index count:

- Use a single `ticket_number` column plus a trigram or expression-index strategy for common suffix/prefix patterns.
- Keep targeted composite indexes only for high-traffic masks.
- Add an in-memory candidate cache keyed by pattern for hot searches.

For this challenge, the full mask-index approach is simple, predictable, and fast.

## Search Algorithm

1. Validate the pattern length is exactly 6 and every character is a digit or `*`.
2. Convert the pattern into equality predicates for non-wildcard positions.
3. Query only available tickets.
4. Order by `id` or a fairness score.
5. Reserve one or more matching rows inside a transaction.

Example for `****23`:

```sql
SELECT id
FROM lottery_tickets
WHERE status = 'available'
  AND d5 = '2'
  AND d6 = '3'
ORDER BY id
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

The query planner can use the `(d5, d6, id)` partial index.

## Duplicate-Prevention Strategy

Use transactional reservation with row-level locks:

```sql
BEGIN;

WITH candidate AS (
  SELECT id
  FROM lottery_tickets
  WHERE status = 'available'
    AND d5 = '2'
    AND d6 = '3'
  ORDER BY id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
UPDATE lottery_tickets t
SET status = 'reserved',
    reserved_by = $user_id,
    reserved_for_pattern = '****23',
    reserved_until = now() + interval '2 minutes'
FROM candidate
WHERE t.id = candidate.id
RETURNING t.*;

COMMIT;
```

`FOR UPDATE SKIP LOCKED` ensures concurrent requests do not block unnecessarily and do not receive the same row. Once a row is locked by one transaction, other transactions skip it and select a different candidate.

Expired reservations are released by a scheduled job:

```sql
UPDATE lottery_tickets
SET status = 'available',
    reserved_by = NULL,
    reserved_for_pattern = NULL,
    reserved_until = NULL
WHERE status = 'reserved'
  AND reserved_until < now();
```

If a user confirms purchase, move `reserved` to `allocated` in a transaction.

## Performance

The dataset size is 10 million rows, but each query constrains at most 6 single-character columns. Selectivity improves with each fixed digit:

| Fixed digits | Approximate candidate count over 10M |
| --- | ---: |
| 1 | 1,000,000 |
| 2 | 100,000 |
| 3 | 10,000 |
| 4 | 1,000 |
| 5 | 100 |
| 6 | 10 |

With composite partial indexes on available rows, PostgreSQL can seek directly into the relevant digit combination and return the first available candidate. The allocation query is `O(log n + k)` where `k` is the number of locked or unavailable candidates skipped.

For the all-wildcard pattern `******`, use an index on `(status, id)` or a partial index on `(id) WHERE status = 'available'`.

## High-Traffic Enhancements

For very high concurrency, add:

- Pattern-level advisory locks only when fairness per pattern is strict and sequential allocation is required.
- Redis sorted sets keyed by wildcard mask and digit values for hot patterns, backed by PostgreSQL confirmation.
- A queue-based allocator per hot pattern to smooth spikes.
- Sharding by ticket number range or hash if write throughput exceeds a single PostgreSQL primary.

The baseline PostgreSQL design is sufficient for 10M records and provides strong correctness with simple operations.

