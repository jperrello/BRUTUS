# Saturn Capability Taxonomy

Saturn is BRUTUS's zero-configuration AI service discovery system. Services advertise their capabilities via mDNS TXT records, enabling automatic discovery and intelligent load balancing across available AI backends.

## Service Discovery

Saturn uses Zeroconf/mDNS to discover services on the local network:

- **Service Type**: `_saturn._tcp`
- **Domain**: `local.`
- **Protocol**: DNS-SD (DNS Service Discovery)

## SaturnService Structure

Each discovered service is represented as a `SaturnService` with the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Service instance name |
| `Host` | string | IP address or hostname |
| `Port` | int | Service port number |
| `Priority` | int | Lower values = higher priority (default: 100) |
| `APIType` | string | API compatibility (default: "openai") |
| `EphemeralKey` | string | Auto-generated API key for authentication |
| `Features` | []string | Capability flags (see Feature Taxonomy below) |
| `APIBase` | string | Override URL for remote services |
| `SaturnVersion` | string | Saturn protocol version |
| `MaxConcurrent` | int | Maximum concurrent requests |
| `CurrentLoad` | int | Current request count |
| `Security` | string | Security level indicator |
| `HealthEndpoint` | string | Custom health check URL |
| `Models` | []string | Available model IDs |
| `GPU` | string | GPU identifier (e.g., "rtx4090") |
| `VRAMGb` | int | GPU VRAM in gigabytes |
| `HealthStatus` | string | "healthy" or degraded state |

## TXT Record Keys

Services advertise capabilities via mDNS TXT records:

| Key | Example Value | Description |
|-----|---------------|-------------|
| `priority` | `50` | Service priority (lower = preferred) |
| `api` | `openai` | API compatibility type |
| `api_base` | `https://api.example.com/v1` | Remote API base URL |
| `ephemeral_key` | `sk-local-xxxx` | Auto-generated bearer token |
| `features` | `chat,tools,streaming` | Comma-separated capability flags |
| `version` | `1.0.0` | Saturn protocol version |
| `max_concurrent` | `4` | Concurrent request limit |
| `current_load` | `2` | Active requests |
| `security` | `local` | Security classification |
| `health_endpoint` | `/health` | Health check path |
| `models` | `gpt-4,claude-3` | Available model IDs |
| `gpu` | `rtx4090` | GPU model identifier |
| `vram_gb` | `24` | Available VRAM |

## Feature Taxonomy

Features are capability flags advertised in the `features` TXT record. These enable filtering during discovery.

### Core Capabilities

| Feature | Description |
|---------|-------------|
| `chat` | Basic chat completion support |
| `tools` | Tool/function calling support |
| `streaming` | Server-sent events streaming |
| `embeddings` | Text embedding generation |
| `vision` | Image input support |
| `audio` | Audio input/output support |

### Advanced Capabilities

| Feature | Description |
|---------|-------------|
| `code` | Optimized for code generation |
| `reasoning` | Extended reasoning/thinking |
| `multimodal` | Multiple input modalities |
| `context-long` | Extended context window (>32K) |
| `context-huge` | Very large context (>100K) |

### Performance Tiers

| Feature | Description |
|---------|-------------|
| `fast` | Low-latency responses |
| `batch` | Optimized for batch processing |
| `gpu-local` | Local GPU inference |
| `cloud` | Cloud-hosted backend |

## Discovery Filters

When discovering services, you can filter by capabilities:

```go
filter := DiscoveryFilter{
    RequiredFeatures: []string{"tools", "streaming"},  // Must have all
    AnyFeatures:      []string{"code", "reasoning"},   // Must have at least one
    APIType:          "openai",                        // Specific API type
    MinPriority:      0,                               // Priority range
    MaxPriority:      100,
    RequireHealthy:   true,                            // Only healthy services
}
```

### Filter Logic

- `RequiredFeatures`: Service must have ALL listed features
- `AnyFeatures`: Service must have AT LEAST ONE listed feature
- `APIType`: Must match exactly
- `MinPriority/MaxPriority`: Priority must be within range (inclusive)
- `RequireHealthy`: HealthStatus must be "healthy" or empty

## Service Selection

When multiple services match, BRUTUS uses a scoring algorithm:

```
score = 100
      - (load_fraction * 50)           // Penalize loaded services
      - 100 (if fully loaded)          // Heavy penalty if at capacity
      + (priority * 10)                // Favor higher priority
      + 20 (if healthy)                // Bonus for confirmed health
      - 30 (if unhealthy)              // Penalty for degraded state
```

The highest-scoring service is selected.

## Service Capacity

Services can advertise capacity information:

- `AvailableCapacity()`: Returns `MaxConcurrent - CurrentLoad`
- `LoadFraction()`: Returns `CurrentLoad / MaxConcurrent` (0.0 to 1.0)

Services with `LoadFraction >= 1.0` are heavily penalized in selection.

## Usage Examples

### Basic Discovery

```go
saturn, err := provider.NewSaturn(ctx, provider.SaturnConfig{
    DiscoveryTimeout: 3 * time.Second,
    Model:           "gpt-4",
    MaxTokens:       4096,
})
```

### Filtered Discovery

```go
saturn, err := provider.NewSaturn(ctx, provider.SaturnConfig{
    DiscoveryTimeout: 3 * time.Second,
    Filter: &provider.DiscoveryFilter{
        RequiredFeatures: []string{"tools"},
        RequireHealthy:   true,
    },
})
```

### Pool Discovery (Multiple Services)

```go
pool, err := provider.NewSaturnPool(ctx, provider.SaturnPoolConfig{
    DiscoveryTimeout: 3 * time.Second,
    MinServices:      2,  // Require at least 2 services
})
```

## Caching

Saturn maintains a global service cache to avoid repeated discovery:

- Default TTL: 5 minutes
- Background refresh runs periodically
- Cache is shared across all Saturn instances in the process

## Fallback Behavior

If Zeroconf discovery fails, Saturn falls back to legacy discovery methods (broadcast UDP, file-based).

## Network Requirements

- mDNS multicast: UDP port 5353
- Service ports: As advertised (typically 8080-9000 range)
- Same local network or mDNS-bridged networks
