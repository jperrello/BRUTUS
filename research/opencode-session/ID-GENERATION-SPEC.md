# OpenCode ID Generation Specification

## Overview

OpenCode uses a custom monotonic ID generation scheme similar to ULID but with:
- **Type prefixes** for schema validation
- **Ascending/descending modes** for sort order control
- **Monotonic counters** for same-millisecond ordering

## ID Format

```
{prefix}_{timestamp_hex}{random_base62}
│        │               │
│        │               └── 14 chars of base62 randomness
│        └── 12 hex chars (6 bytes = 48 bits)
└── Type prefix (3-4 chars)
```

### Example IDs

```
ses_fec3d8a1b2c3ABCDEFghijklmn  (session, descending - newest first)
msg_01932a4b5c6dXYZ123abcdefg  (message, ascending - chronological)
prt_01932a4b5c6e789012qrstuv  (part, ascending)
```

## Prefixes

| Prefix | Entity | Ordering |
|--------|--------|----------|
| `ses` | Session | Descending (newest first for listing) |
| `msg` | Message | Ascending (chronological) |
| `prt` | Part | Ascending (chronological) |
| `per` | Permission | Ascending |
| `que` | Question | Ascending |
| `usr` | User | Ascending |
| `pty` | PTY | Ascending |
| `tool` | Tool | Ascending |

## Timestamp Encoding

### 48-bit Packed Format

```
┌────────────────────────────────────────────────┐
│           48 bits total                        │
├────────────────────────────────┬───────────────┤
│     36 bits: timestamp         │ 12 bits: ctr  │
│     (milliseconds since epoch) │ (0-4095)      │
└────────────────────────────────┴───────────────┘
```

### Encoding Process

```typescript
// Step 1: Get timestamp and increment counter
const currentTimestamp = Date.now()  // ms since epoch
if (currentTimestamp !== lastTimestamp) {
  lastTimestamp = currentTimestamp
  counter = 0
}
counter++

// Step 2: Pack into 48 bits
// Multiply by 0x1000 (4096) shifts timestamp left 12 bits
let now = BigInt(currentTimestamp) * BigInt(0x1000) + BigInt(counter)
// now = (timestamp << 12) | counter

// Step 3: Optionally invert for descending order
if (descending) {
  now = ~now  // Bitwise NOT: larger values become smaller
}

// Step 4: Convert to 6 bytes
const timeBytes = Buffer.alloc(6)
for (let i = 0; i < 6; i++) {
  timeBytes[i] = Number((now >> BigInt(40 - 8 * i)) & BigInt(0xff))
}
// Extracts bytes from most significant to least significant

// Step 5: Hex encode
const hex = timeBytes.toString("hex")  // 12 hex characters
```

### Decoding (Ascending Only)

```typescript
function timestamp(id: string): number {
  const prefix = id.split("_")[0]
  const hex = id.slice(prefix.length + 1, prefix.length + 13)
  const encoded = BigInt("0x" + hex)
  return Number(encoded / BigInt(0x1000))  // Divide to remove counter bits
}
```

**Note**: Decoding descending IDs requires bitwise NOT first.

## Random Suffix

### Base62 Alphabet

```typescript
const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
// 62 characters: 0-9, A-Z, a-z
```

### Generation

```typescript
function randomBase62(length: number): string {
  let result = ""
  const bytes = randomBytes(length)  // Cryptographic random
  for (let i = 0; i < length; i++) {
    result += chars[bytes[i] % 62]  // Mod 62 for alphabet selection
  }
  return result
}
```

### Entropy Analysis

- 14 characters of base62 = `log2(62^14)` = ~83.4 bits of randomness
- Combined with timestamp: effectively unique globally
- Collision probability: negligible for practical use

## Monotonic Guarantee

### Problem

Multiple IDs generated in the same millisecond could have identical timestamps.

### Solution

Counter increments for each ID within the same millisecond:

```typescript
let lastTimestamp = 0
let counter = 0

// On each ID generation:
if (currentTimestamp !== lastTimestamp) {
  lastTimestamp = currentTimestamp
  counter = 0  // Reset counter for new millisecond
}
counter++  // Increment: 1, 2, 3... up to 4095 per ms
```

### Capacity

- 12-bit counter: 4096 IDs per millisecond
- At 4000 IDs/ms = 4,000,000 IDs/second
- Far exceeds practical usage

## Ascending vs Descending

### Ascending (Default)

- IDs increase with time
- Lexicographic sort = chronological order
- Used for: messages, parts, permissions

```
msg_000000000001... (oldest)
msg_000000000002...
msg_000000000003... (newest)
```

### Descending

- IDs decrease with time (via bitwise NOT)
- Lexicographic sort = reverse chronological
- Used for: sessions (newest first in listings)

```
ses_ffffffffff01... (newest)
ses_ffffffffff00...
ses_ffffffffffff... (oldest)
```

### Bitwise NOT Effect

```
Original:  0x0193 2A4B 5C6D (ascending)
Inverted:  0xFE6C D5B4 A392 (descending)

When timestamps increase:
  Ascending:  values get larger (0x01 → 0x02 → 0x03)
  Descending: values get smaller (0xFE → 0xFD → 0xFC)
```

## Zod Schema Validation

```typescript
Identifier.schema(prefix: keyof typeof prefixes) {
  return z.string().startsWith(prefixes[prefix])
}

// Usage
const SessionID = Identifier.schema("session")  // Validates "ses_..."
const MessageID = Identifier.schema("message")  // Validates "msg_..."
```

## Comparison with ULID

| Feature | OpenCode IDs | ULID |
|---------|--------------|------|
| Format | `{prefix}_{ts}{random}` | `{ts}{random}` |
| Timestamp | 48-bit (36+12 counter) | 48-bit |
| Random | 14 chars base62 (~83 bits) | 80 bits |
| Encoding | hex + base62 | Crockford base32 |
| Ordering | Ascending or descending | Ascending only |
| Type safety | Prefix-based | None |

## Implementation Notes

### Thread Safety

The current implementation uses module-level `lastTimestamp` and `counter` variables. In single-threaded JavaScript/Bun, this is safe. For multi-threaded environments, atomic operations would be needed.

### Clock Skew

If the system clock moves backward, the counter will be reset on the next forward tick. This could theoretically produce non-monotonic IDs, but:
1. The random suffix ensures uniqueness
2. Clock skew is rare in practice
3. The 12-bit counter provides buffer for minor adjustments

### Storage Efficiency

- Prefix: 3-4 bytes
- Separator: 1 byte
- Timestamp (hex): 12 bytes
- Random (base62): 14 bytes
- Total: ~30-31 bytes per ID

Compared to UUIDs (36 bytes with hyphens), slightly more compact while providing ordering guarantees.
