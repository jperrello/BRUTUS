# Saturn Beacon Configuration Standard

## Overview

The `saturn-beacon.yaml` configuration file defines how a Saturn beacon advertises AI services on the local network. This standard enables:
- Consistent configuration across beacon implementations
- Portable configurations between systems
- Clear documentation of all available options

## File Location

Beacons should look for configuration in this order:
1. Path specified via `--config` flag
2. `./saturn-beacon.yaml` (current directory)
3. `~/.config/saturn/beacon.yaml` (user config)
4. `/etc/saturn/beacon.yaml` (system config, Linux/macOS)
5. `%APPDATA%\saturn\beacon.yaml` (system config, Windows)

## Configuration Schema

### Minimal Configuration

```yaml
saturn_version: "1.1"

service:
  name: ollama-local
```

### Full Configuration Reference

```yaml
# Saturn protocol version (required)
saturn_version: "1.1"

# Service identity and discovery priority
service:
  # Unique name for this service instance on the network
  # Used in mDNS instance name: {name}._saturn._tcp.local
  # Must be: lowercase, alphanumeric + hyphens, 1-63 chars
  name: ollama-workstation

  # Discovery priority (lower = higher priority)
  # Range: 0-100, default: 50
  # 0-9: Production/preferred services
  # 10-49: Normal priority
  # 50-89: Fallback services
  # 90-100: Development/testing
  priority: 10

  # Optional human-readable description
  description: "Primary workstation Ollama server"

# API configuration
api:
  # API compatibility type
  # Supported: openai (default), anthropic, ollama
  type: openai

  # Base URL for the API
  # Default: http://localhost:11434 (Ollama default)
  base_url: http://localhost:11434/v1

  # For remote/proxy beacons: external API URL
  # If set, the beacon proxies to this URL
  remote_url: null

# Features advertised to clients
# Standard feature tokens:
features:
  - streaming     # SSE streaming responses
  - tools         # Function/tool calling
  - vision        # Image inputs
  - embeddings    # /v1/embeddings endpoint
  - audio         # Audio transcription/TTS
  - multimodal    # Multiple input modalities
  - code          # Code-optimized models

# Security settings
security:
  # Security mode: none, ephemeral_key, api_key
  # none: No authentication required
  # ephemeral_key: Generate session-scoped key per discovery
  # api_key: Require static API key (must provide in key_file)
  mode: none

  # Path to file containing API key (for api_key mode)
  key_file: null

  # Allowed client IP ranges (CIDR notation)
  # Empty = allow all local network
  allowed_networks:
    - 192.168.0.0/16
    - 10.0.0.0/8

# Load and capacity hints for client-side load balancing
capacity:
  # Maximum concurrent requests the server can handle
  # 0 = unlimited/unknown
  max_concurrent: 10

  # Current load (updated dynamically if beacon supports)
  # Clients use this for load-balanced selection
  current_load: 0

# Hardware information for capability matching
hardware:
  # GPU type: cuda, rocm, metal, cpu, none
  gpu: cuda

  # VRAM in GB (for GPU scheduling decisions)
  vram_gb: 24

  # Optional: specific GPU model
  gpu_model: "RTX 4090"

# Available models on this service
# List of model IDs available through this beacon
models:
  - llama3.1:8b
  - codellama:7b
  - llama3.1:70b

# Health check configuration
health:
  # Health endpoint path
  endpoint: /v1/health

  # Health check interval (seconds)
  # Beacon will perform self-checks at this interval
  interval: 30

  # Timeout for health check requests (seconds)
  timeout: 5

# mDNS announcement settings
announce:
  # Time-to-live for mDNS records (seconds)
  # Clients will re-query after this expires
  ttl: 600

  # How often to refresh announcement (seconds)
  # Should be less than TTL
  refresh: 300

  # Network interface to announce on
  # null = all interfaces
  interface: null

# Logging configuration
logging:
  # Log level: debug, info, warn, error
  level: info

  # Log file path (null = stdout)
  file: null
```

## TXT Record Mapping

When announced via mDNS, the configuration maps to DNS TXT records:

| YAML Path | TXT Record Key | Example Value |
|-----------|----------------|---------------|
| `saturn_version` | `saturn_version` | `1.1` |
| `service.priority` | `priority` | `10` |
| `api.type` | `api` | `openai` |
| `api.base_url` | `api_base` | `http://localhost:11434/v1` |
| `api.remote_url` | `remote_url` | `https://api.openai.com/v1` |
| `features` | `features` | `streaming,tools,vision` |
| `security.mode` | `security` | `ephemeral_key` |
| (generated) | `ephemeral_key` | `sk-local-abc123` |
| `capacity.max_concurrent` | `max_concurrent` | `10` |
| `capacity.current_load` | `current_load` | `3` |
| `hardware.gpu` | `gpu` | `cuda` |
| `hardware.vram_gb` | `vram_gb` | `24` |
| `models` | `models` | `llama3.1:8b,codellama:7b` |
| `health.endpoint` | `health_endpoint` | `/v1/health` |

## Configuration Examples

### Local Ollama Server

```yaml
saturn_version: "1.1"

service:
  name: ollama-desktop
  priority: 10

api:
  type: openai
  base_url: http://localhost:11434/v1

features:
  - streaming
  - tools

hardware:
  gpu: cuda
  vram_gb: 12

models:
  - llama3.1:8b
  - codellama:7b
```

### Remote API Proxy

```yaml
saturn_version: "1.1"

service:
  name: openrouter-proxy
  priority: 50
  description: "Proxy to OpenRouter for cloud models"

api:
  type: openai
  remote_url: https://openrouter.ai/api/v1

security:
  mode: api_key
  key_file: ~/.config/saturn/openrouter.key

features:
  - streaming
  - tools
  - vision
  - embeddings

capacity:
  max_concurrent: 0
```

### Multi-GPU Inference Server

```yaml
saturn_version: "1.1"

service:
  name: inference-server
  priority: 5
  description: "Dedicated inference server with 2x A100"

api:
  type: openai
  base_url: http://192.168.1.100:8000/v1

features:
  - streaming
  - tools
  - vision
  - multimodal

capacity:
  max_concurrent: 20

hardware:
  gpu: cuda
  vram_gb: 160
  gpu_model: "2x A100 80GB"

models:
  - llama3.1:70b
  - llama3.1:405b
  - deepseek-coder:33b

health:
  endpoint: /health
  interval: 15
```

### Development/Testing Beacon

```yaml
saturn_version: "1.1"

service:
  name: dev-test
  priority: 99
  description: "Development testing only"

api:
  type: openai
  base_url: http://localhost:3000/v1

features:
  - streaming

capacity:
  max_concurrent: 1

logging:
  level: debug
```

## Validation Rules

### Service Name
- Must match regex: `^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`
- Cannot start or end with hyphen
- Maximum 63 characters

### Priority
- Integer range: 0-100
- 0 = highest priority
- 100 = lowest priority

### Features
- Must be from standard feature set or prefixed with `x-` for custom
- Examples: `streaming`, `tools`, `x-custom-feature`

### URLs
- Must be valid HTTP/HTTPS URLs
- `base_url` should include protocol and optional port
- Should not include trailing slash

## Implementation Notes

### For Beacon Developers

1. Parse configuration with YAML library
2. Validate all fields before starting
3. Generate ephemeral key if `security.mode: ephemeral_key`
4. Register mDNS service with TXT records
5. Start health check loop if configured
6. Handle graceful shutdown with "goodbye" packets

### For Client Developers

1. Parse TXT records from mDNS discovery
2. Filter by required features
3. Sort by priority, then load factor
4. Prefer services with `health_endpoint` that return healthy
5. Cache services for `ttl` duration
6. Handle ephemeral key renewal on 401 errors

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-01 | Initial release |
| 1.1 | 2026-01 | Added capacity, hardware, models fields |
