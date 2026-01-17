# Saturn Protocol Analysis & Improvement Roadmap

This document analyzes the Saturn protocol based on academic research on mDNS/DNS-SD and proposes improvements for both BRUTUS and Saturn as a general-purpose AI service discovery protocol.

---

## Table of Contents
1. [Background: mDNS/DNS-SD Fundamentals](#background)
2. [Current Saturn Implementation](#current-implementation)
3. [Literature Review Key Insights](#literature-review)
4. [Improvements for BRUTUS](#improvements-brutus)
5. [Improvements for Saturn Protocol](#improvements-protocol)
6. [Implementation Roadmap](#roadmap)

---

## Background: mDNS/DNS-SD Fundamentals <a name="background"></a>

### Why Saturn Uses This

Saturn leverages mDNS/DNS-SD because:
1. **Zero-configuration**: Services auto-discover without setup files
2. **Decentralized**: No central registry point of failure
3. **Standard**: Works with Bonjour (macOS/Windows), Avahi (Linux), and embedded systems
4. **Metadata-rich**: TXT records carry ephemeral keys and capabilities

---

## Current Saturn Implementation <a name="current-implementation"></a>

### Service Definition (from `discovery.go`)

```
Service Type: _saturn._tcp.local
```

### TXT Record Fields

| Field | Purpose | Example |
|-------|---------|---------|
| `priority` | Service preference (lower = better) | `10` |
| `api` | API compatibility type | `openai` |
| `api_base` | Remote API URL (for proxies) | `https://openrouter.ai/api/v1` |
| `ephemeral_key` | Session-scoped API credential | `sk-xxx` |
| `features` | Comma-separated capabilities | `streaming,tools,vision` |

### Current Discovery Flow

```
1. Client runs: dns-sd -B _saturn._tcp local.
2. Parses instance names from browse output
3. For each instance: dns-sd -L <instance> _saturn._tcp local.
4. Extracts host:port and TXT records
5. Performs health check on /v1/health
6. Selects highest-priority healthy service
```

### Limitations Identified

1. **Shell-out dependency**: Relies on `dns-sd` CLI tool (Windows-specific)
2. **No native Go mDNS**: Would break on Linux without Avahi shim
3. **Single-service selection**: No load balancing or failover rotation
4. **No service versioning**: Can't distinguish Saturn v1 vs v2 beacons
5. **Privacy exposure**: Service names and metadata visible to all LAN devices
6. **No capability negotiation**: Client can't request specific features

---

## Literature Review Key Insights <a name="literature-review"></a>

### From "Bonjour Contiki" (Klauck & Kirsche, ADHOC-NOW 2012)

**Key Findings for Resource-Constrained Environments:**

1. **Memory Efficiency**: Their uBonjour implementation achieved 3.82 KB ROM / 0.3 KB RAM - proving mDNS can run on tiny microcontrollers. Saturn beacons could take up much less memory on a router. 

2. **One-Way Traffic (OWT) Optimization**: For constrained devices, a "passive mode" where devices only:
   - Publish services periodically
   - Respond to incoming queries
   - Don't actively browse for other services

   **Saturn Application**: Beacons (Ollama servers, etc.) should operate in OWT mode. Only clients (BRUTUS) actively browse.

3. **Message Size Constraints**: DNS records must fit in single packets. For Saturn, this means:
   - TXT record values should be compact
   - Consider splitting large metadata across multiple TXT keys
   - Total TXT data should stay under ~200 bytes for universal compatibility

4. **TTL Optimization**: Default 120-second TTL causes excessive traffic. Recommendation: Increase to 600+ seconds for stable services, but implement explicit "goodbye" packets when services stop.

5. **Known-Answer Suppression**: Reduces redundant responses but requires caching. Trade-off for memory-constrained beacons.

### From ESP32 mDNS Guide

**Practical Implementation Patterns:**

1. **Service Properties via TXT**: The ESP32 approach of adding key-value pairs:
   ```cpp
   MDNS.addServiceTxt("http", "tcp", "prop1", "test1");
   ```
   Maps directly to Saturn's current design. TXT records are the right place for metadata.

2. **Hostname + Service Separation**: Devices set both:
   - Hostname: `esp32.local` (for direct access)
   - Service: `esp32._http._tcp.local` (for discovery)

   **Saturn should do the same**: Beacons should have memorable hostnames like `ollama.local` alongside the service registration.

3. **Python Zeroconf Library**: The guide shows Python clients using `zeroconf` library - this same library could replace BRUTUS's shell-out to `dns-sd`.

### Privacy Considerations (General mDNS Literature)

**Known Privacy Issues:**

1. **Service Enumeration**: Anyone on the LAN can discover all Saturn services
2. **Metadata Leakage**: TXT records (including API keys) are broadcast in plaintext
3. **Device Fingerprinting**: Service names/features reveal infrastructure details

**Mitigation Strategies:**

1. **Ephemeral Keys**: Saturn already does this - keys are session-scoped
2. **Obfuscated Instance Names**: Use random UUIDs instead of descriptive names
3. **Private DNS-SD Extensions**: Some proposals use encryption for TXT records
4. **Network Segmentation**: Run Saturn discovery on isolated VLAN

---

## Improvements for BRUTUS <a name="improvements-brutus"></a>

### 1. Native Go mDNS Library

**Current Problem**: Shells out to `dns-sd` command.

**Solution**: Use `github.com/grandcat/zeroconf` or `github.com/hashicorp/mdns`:

```go
// Proposed: Native Go discovery
resolver, _ := zeroconf.NewResolver(nil)
entries := make(chan *zeroconf.ServiceEntry)
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

go func() {
    for entry := range entries {
        // entry.ServiceInstanceName, entry.Port, entry.Text
    }
}()
resolver.Browse(ctx, "_saturn._tcp", "local.", entries)
```

**Benefits**:
- Cross-platform without external dependencies
- Proper error handling
- Async/streaming results

### 2. Service Caching with TTL

**Current Problem**: Full discovery on every startup.

**Solution**: Implement a service cache:

```go
type ServiceCache struct {
    services map[string]CachedService
    mu       sync.RWMutex
}

type CachedService struct {
    Service   SaturnService
    ExpiresAt time.Time
    LastCheck time.Time
}
```

- Cache discovered services for TTL duration
- Background refresh before expiration
- Immediate invalidation on health check failure

### 3. Capability-Based Selection

**Current Problem**: Only uses priority for service selection.

**Enhancement**: Filter by required capabilities:

```go
type DiscoveryFilter struct {
    RequiredFeatures []string // e.g., ["streaming", "tools"]
    MinPriority      int
    APIType          string   // e.g., "openai"
}

func DiscoverSaturnFiltered(ctx context.Context, filter DiscoveryFilter) ([]SaturnService, error)
```

### 4. Fallback Chain

**Current Problem**: Single service selection, basic failover.

**Enhancement**: Maintain ordered fallback chain:

```go
type SaturnPool struct {
    services []SaturnService
    current  int
    mu       sync.Mutex
}

func (p *SaturnPool) Next() *SaturnService {
    // Round-robin or failover logic
}
```

### 5. Connection Pooling

**Current Problem**: New HTTP client per request.

**Solution**: Reuse connections with keep-alive:

```go
transport := &http.Transport{
    MaxIdleConns:        10,
    IdleConnTimeout:     90 * time.Second,
    MaxConnsPerHost:     5,
}
```

---

## Improvements for Saturn Protocol <a name="improvements-protocol"></a>

### 2. Capability Taxonomy

**Problem**: Freeform `features` field is ambiguous.

**Solution**: Define standard capability tokens:

| Capability | Meaning |
|------------|---------|
| `streaming` | Supports SSE streaming responses |
| `tools` | Supports function/tool calling |
| `vision` | Accepts image inputs |
| `embeddings` | Provides /v1/embeddings endpoint |
| `audio` | Supports audio transcription |
| `multimodal` | Multiple input modalities |

**TXT format**: `features=streaming,tools,vision`

### 3. Load Balancing Hints

**Problem**: No way to express server capacity.

**Solution**: Add load/capacity fields:

```
TXT max_concurrent=10
TXT current_load=3
```

Clients can distribute requests across servers proportionally.
```

### 6. Health Metadata

**Problem**: Health is binary pass/fail.

**Solution**: Rich health information in TXT:

```
TXT health_endpoint=/v1/health
TXT models=llama3.1,codellama
TXT gpu=cuda
TXT vram_gb=24
```

### 7. Beacon Configuration Standard

Define a standard beacon config file:

```yaml
# saturn-beacon.yaml
service:
  name: ollama-workstation
  priority: 10

api:
  type: openai
  base_url: http://localhost:11434

features:
  - streaming
  - tools

security:
  type: none

announce:
  ttl: 600
  refresh: 300
```


## Implementation Roadmap <a name="roadmap"></a>

### Phase 1: BRUTUS Improvements (Near-term)

| Task | Priority | Effort |
|------|----------|--------|
| Replace `dns-sd` with Go zeroconf library | High | Medium |
| Add service caching with TTL | Medium | Low |
| Implement capability filtering | Medium | Low |
| Add connection pooling | Low | Low |

### Phase 2: Protocol Enhancements (Medium-term)

| Task | Priority | Effort |
|------|----------|--------|
| Define `saturn_version` field | High | Trivial |
| Standardize capability taxonomy | High | Low |
| Document service subtypes | Medium | Low |
| Add load balancing hints | Low | Low |

### Phase 3: Ecosystem (Long-term)

| Task | Priority | Effort |
|------|----------|--------|
| Reference beacon implementation (Go) | High | Medium |
| Reference beacon implementation (Rust) | Medium | Medium |
| ESP32 Saturn beacon | Medium | High |
| Wide-Area Saturn spec | Low | High |


---

## References

1. Klauck, R. & Kirsche, M. (2012). "Bonjour Contiki: A Case Study of a DNS-based Discovery Service for the Internet of Things." ADHOC-NOW 2012.

2. Cheshire, S. & Krochmal, M. (2013). "Multicast DNS." RFC 6762.

3. Cheshire, S. & Krochmal, M. (2013). "DNS-Based Service Discovery." RFC 6763.

4. ESP32 mDNS Documentation. Espressif Systems.

---

## Appendix: Example TXT Record (Full Saturn v1.1)

```
Instance: ollama-workstation._saturn._tcp.local
Host: ollama-workstation.local:11434

TXT Records:
  saturn_version=1.1
  priority=10
  api=openai
  features=streaming,tools,vision
  ephemeral_key=sk-local-abc123
  models=llama3.1:8b,codellama:7b
  gpu=cuda
  max_concurrent=5
  health_endpoint=/v1/health
```
