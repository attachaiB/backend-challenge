# Lottery Search System: Design Proposal

Design only, no code, as the challenge asks.

## 1. Problem summary

- 10 million tickets. Each ticket has a 6-digit number (`000000`–`999999`).
- Users search with a 6-character pattern of digits and `*`, for example
  `****23`, `1****5`, `123***`.
- Two users searching the same pattern at the same time must never receive the
  same ticket.
- The search must stay fast at 10M+ rows.

One observation shapes the design: there are only **1,000,000 possible
numbers** but 10 million tickets, so each number exists about 10 times (for
example as different sets or series). A pattern matches numbers, and each
number then maps to about 10 tickets.

## 2. Assumptions

- A search shows the user a small page of tickets (for example 10) that they
  can buy, not every match. Returning thousands of rows to one user is not
  useful and would lock tickets nobody can look at.
- "Not returned to multiple users at the same time" means: once a ticket is
  shown to one user, it is **reserved** for that user for a short time (for
  example 10 minutes). It goes back to the pool if they do not buy it.
- A ticket is either `available`, `reserved` or `sold`.

## 3. Storage choice: PostgreSQL

| Need | Why PostgreSQL fits |
| --- | --- |
| Never give one ticket to two users | Row locks and `SELECT ... FOR UPDATE SKIP LOCKED` give atomic allocation without a separate lock service. |
| Reservation then purchase | Transactions: reserve, then confirm or release, with clear states. |
| Fast lookups on 10M rows | B-tree and partial indexes. 10M small rows is about 1 GB with indexes, which fits in memory on one server. |
| Operations | Well known, easy to back up, replicate and monitor. One system instead of a database plus a separate lock or cache layer. |

I considered:

- **MongoDB**: `findOneAndUpdate` is atomic for one document, but reserving a
  page of 10 tickets means 10 separate atomic calls, and there is no equivalent
  of `SKIP LOCKED`, so concurrent users retry on the same documents.
- **Redis only**: very fast, but tickets are money. I want the source of truth
  to be durable and transactional. Redis is useful later as an optional search
  accelerator (section 8).

## 4. Data model

```sql
CREATE TABLE tickets (
    id             BIGSERIAL PRIMARY KEY,
    number         CHAR(6) COLLATE "C" NOT NULL,   -- '012345'
    status         TEXT        NOT NULL DEFAULT 'available',  -- available | reserved | sold
    reserved_by    BIGINT,
    reserved_until TIMESTAMPTZ,
    sold_to        BIGINT
);

-- Only available tickets are searched, so the index holds only those rows.
-- It shrinks as tickets are reserved or sold, and searches never read sold tickets.
CREATE INDEX tickets_available_number
    ON tickets (number)
    WHERE status = 'available';
```

`COLLATE "C"` makes the column compare byte by byte, so both exact lookups and
a prefix such as `123` use the index (a prefix becomes a simple range scan).

## 5. Search algorithm

A pattern has `k` wildcards and `6 - k` fixed digits. The algorithm picks one of
two strategies based on `k`, because the right strategy depends on how
selective the pattern is.

### Strategy A: few wildcards (k ≤ 3), enumerate the numbers

The pattern matches at most `10^k` numbers, so at most 1,000. Generate them all
in the application and look them up directly:

```
1*3*5*  → 100 numbers: 103050, 103051, ..., 193959
```

```sql
WHERE status = 'available' AND number = ANY($numbers)
```

Each number is one index lookup. 1,000 lookups on an in-memory B-tree take
a few milliseconds. This also covers patterns with no fixed prefix such as
`*1234*`, which a normal index could not use.

### Strategy B: many wildcards (k ≥ 4), scan with a filter

At least 1 in 100 tickets match (`10^k / 10^6` with `k ≥ 4`). Matches are
common, so scanning the available index and keeping the matching rows finds a
page of 10 matches after about `10 × 100 = 1,000` rows on average:

```sql
WHERE status = 'available' AND number LIKE $like   -- '****23' → '____23'
LIMIT 10
```

If the pattern starts with fixed digits (`12****`), PostgreSQL turns the
`LIKE` into a range scan on the index and reads only matching rows.

### Why the split at k = 3 / k = 4

The two strategies cover each other's weak case:

| k (wildcards) | Numbers matched | Strategy | Rows read for a page of 10 |
| --- | --- | --- | --- |
| 0–3 | 1–1,000 | A: enumerate | ≤ 1,000 index lookups |
| 4–6 | 10,000–1,000,000 | B: filtered scan | about 1,000 on average |

Neither strategy ever reads all 10M rows to fill a page. The one slow case is a
pattern whose matches are almost all reserved or sold: the partial index
already excludes them, so this is only slow while tickets are reserved but not
yet released.

## 6. Allocation without duplicates

Search and reserve happen in **one statement**, so there is no gap between
"find" and "take" in which another user can take the same ticket:

```sql
WITH picked AS (
    SELECT id
    FROM tickets
    WHERE status = 'available'
      AND number = ANY($numbers)        -- or: number LIKE $like (strategy B)
    LIMIT 10
    FOR UPDATE SKIP LOCKED
)
UPDATE tickets t
SET status = 'reserved',
    reserved_by = $user_id,
    reserved_until = now() + interval '10 minutes'
FROM picked
WHERE t.id = picked.id
RETURNING t.id, t.number;
```

How this prevents duplicates when users A and B search `****23` at the same time:

1. A's statement locks the rows it picks.
2. B's statement reaches the same rows, sees they are locked, and **skips**
   them (`SKIP LOCKED`) instead of waiting. It takes the next matching rows.
3. A commits; those rows are now `reserved` and leave the partial index.

So A and B always get different tickets, and B is never blocked by A.

### Purchase and release

- **Buy**: `UPDATE tickets SET status = 'sold', sold_to = $user
  WHERE id = $id AND reserved_by = $user AND reserved_until > now()`.
  If no row is updated, the reservation expired and the user must search again.
- **Release**: when the user leaves, or a background job every minute runs
  `UPDATE tickets SET status = 'available', reserved_by = NULL, reserved_until = NULL
  WHERE status = 'reserved' AND reserved_until < now()`.

### Avoiding a hot spot

Without care, every user searching `****23` scans the same first rows of the
index and has to skip each other's locks. To spread them out:

- Strategy A: shuffle the candidate numbers before the query.
- Strategy B: start the scan at a random point, for example
  `AND number >= $random_start`, and wrap around if the page is not full.

## 7. Performance analysis

Estimates for 10M tickets on one PostgreSQL server with the index in memory:

| Operation | Work | Expected time |
| --- | --- | --- |
| Search + reserve, k ≤ 3 | ≤ 1,000 index lookups + 10 row updates | a few ms |
| Search + reserve, k ≥ 4 | about 1,000 index entries read + 10 row updates | a few ms |
| Purchase | 1 primary-key update | < 1 ms |
| Release job | indexed update of expired rows | runs in the background |

Storage: about 60 bytes per row plus the index, roughly 1 GB in total.

Tradeoffs:

- **Every search writes** (it reserves). This is the price of the "no
  duplicates at the same time" rule. Writes are small (10 rows) and spread out
  by the random start.
- **The partial index changes on every reserve and release.** PostgreSQL
  handles this, but autovacuum must keep up; I would monitor index bloat.
- **One primary database** handles all writes. For much higher traffic I would
  partition `tickets` by the last digit of the number (10 partitions) so
  searches and locks spread over 10 smaller tables, and use read replicas only
  for read-only screens such as "how many tickets match".

## 8. Optional: in-memory bitmap index at larger scale

If search traffic outgrows PostgreSQL, a faster search index can sit in front
of it without changing the allocation rule:

- Keep 60 bitmaps (6 positions × 10 digits), each with one bit per ticket:
  bit `i` of bitmap `pos2=3` is set when ticket `i` has `3` in position 2.
  Plus one bitmap of available tickets.
- A pattern is an `AND` of the bitmaps for its fixed digits and the
  available bitmap. 10M bits is 1.25 MB, so an `AND` takes well under a
  millisecond. Redis supports this directly with `BITOP AND` and `BITPOS`.
- The bitmap only suggests candidate ticket IDs. The reservation is still the
  PostgreSQL `UPDATE ... FOR UPDATE SKIP LOCKED` statement with
  `id = ANY($candidates)`, so correctness never depends on the bitmap being
  perfectly up to date.

I would start without this. The PostgreSQL design above meets the stated scale
and is simpler to run.

## 9. Summary

- **Storage**: PostgreSQL, one `tickets` table, partial index on available numbers.
- **Search**: enumerate up to 1,000 numbers when the pattern has ≤ 3
  wildcards; otherwise scan the index with a `LIKE` filter and `LIMIT`.
- **No duplicates**: search and reserve in one statement with
  `FOR UPDATE SKIP LOCKED`, reservations expire after a timeout.
- **Performance**: a few milliseconds per search at 10M tickets, without reading
  the whole table in any case.
