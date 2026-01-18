# OpenCode Provider Subsystem Specification

**Researcher**: Claude Agent
**Date**: 2026-01-17
**Subject**: Provider management system in OpenCode
**Source**: `packages/opencode/src/provider/`

---

## Executive Summary

OpenCode's provider subsystem manages connections to 20+ LLM providers through a unified interface. It handles SDK instantiation, model discovery, authentication, request transformation, and response normalization. The architecture uses lazy initialization, caching, and instance-based state management.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Provider Namespace                       │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   models.ts  │  │  provider.ts │  │   transform.ts   │  │
│  │ (ModelsDev)  │  │   (Main)     │  │ (Transformers)   │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                 │                    │            │
│         └────────┬────────┴────────────────────┘            │
│                  ▼                                          │
│         ┌────────────────┐                                  │
│         │ Instance State │                                  │
│         │  (Singleton)   │                                  │
│         └────────┬───────┘                                  │
│                  │                                          │
│    ┌─────────────┼─────────────┬─────────────┐             │
│    ▼             ▼             ▼             ▼             │
│ ┌──────┐   ┌──────────┐  ┌──────────┐  ┌───────────┐      │
│ │ SDK  │   │ Providers│  │ Languages│  │ModelLoaders│      │
│ │ Map  │   │   Map    │  │   Map    │  │   Map     │      │
│ └──────┘   └──────────┘  └──────────┘  └───────────┘      │
└─────────────────────────────────────────────────────────────┘
```

---

## Data Structures

### Model Schema (Zod)

```typescript
Model = z.object({
  id: z.string(),
  providerID: z.string(),
  api: z.object({
    id: z.string(),        // API-level model ID
    url: z.string(),       // Base URL for API
    npm: z.string(),       // SDK package name
  }),
  name: z.string(),
  family: z.string().optional(),
  capabilities: z.object({
    temperature: z.boolean(),
    reasoning: z.boolean(),
    attachment: z.boolean(),
    toolcall: z.boolean(),
    input: z.object({
      text: z.boolean(),
      audio: z.boolean(),
      image: z.boolean(),
      video: z.boolean(),
      pdf: z.boolean(),
    }),
    output: z.object({
      text: z.boolean(),
      audio: z.boolean(),
      image: z.boolean(),
      video: z.boolean(),
      pdf: z.boolean(),
    }),
    interleaved: z.union([
      z.boolean(),
      z.object({
        field: z.enum(["reasoning_content", "reasoning_details"]),
      }),
    ]),
  }),
  cost: z.object({
    input: z.number(),   // per million tokens
    output: z.number(),
    cache: z.object({
      read: z.number(),
      write: z.number(),
    }),
    experimentalOver200K: z.object({...}).optional(),
  }),
  limit: z.object({
    context: z.number(),
    input: z.number().optional(),
    output: z.number(),
  }),
  status: z.enum(["alpha", "beta", "deprecated", "active"]),
  options: z.record(z.string(), z.any()),
  headers: z.record(z.string(), z.string()),
  release_date: z.string(),
  variants: z.record(z.string(), z.record(z.string(), z.any())).optional(),
})
```

### Provider Info Schema

```typescript
Info = z.object({
  id: z.string(),
  name: z.string(),
  source: z.enum(["env", "config", "custom", "api"]),
  env: z.string().array(),      // Environment variable names
  key: z.string().optional(),   // API key if single env var
  options: z.record(z.string(), z.any()),
  models: z.record(z.string(), Model),
})
```

---

## Bundled Providers

OpenCode bundles 21 provider SDKs:

| Provider | NPM Package | Custom Loader |
|----------|-------------|---------------|
| Amazon Bedrock | `@ai-sdk/amazon-bedrock` | Yes (region routing) |
| Anthropic | `@ai-sdk/anthropic` | Yes (beta headers) |
| Azure | `@ai-sdk/azure` | Yes (responses vs chat) |
| Google | `@ai-sdk/google` | No |
| Google Vertex | `@ai-sdk/google-vertex` | Yes (project/location) |
| Google Vertex Anthropic | `@ai-sdk/google-vertex/anthropic` | Yes |
| OpenAI | `@ai-sdk/openai` | Yes (responses mode) |
| OpenRouter | `@openrouter/ai-sdk-provider` | Yes (headers) |
| GitHub Copilot | `@ai-sdk/github-copilot` (custom) | Yes |
| xAI | `@ai-sdk/xai` | No |
| Mistral | `@ai-sdk/mistral` | No |
| Groq | `@ai-sdk/groq` | No |
| DeepInfra | `@ai-sdk/deepinfra` | No |
| Cerebras | `@ai-sdk/cerebras` | Yes (headers) |
| Cohere | `@ai-sdk/cohere` | No |
| Gateway | `@ai-sdk/gateway` | No |
| TogetherAI | `@ai-sdk/togetherai` | No |
| Perplexity | `@ai-sdk/perplexity` | No |
| Vercel | `@ai-sdk/vercel` | Yes (headers) |
| GitLab | `@gitlab/gitlab-ai-provider` | Yes |
| OpenAI Compatible | `@ai-sdk/openai-compatible` | No |

---

## Custom Loaders

Custom loaders modify SDK initialization per provider:

### Anthropic
```typescript
{
  autoload: false,
  options: {
    headers: {
      "anthropic-beta": "claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"
    }
  }
}
```

### Amazon Bedrock (Region Routing)
```typescript
// Model ID prefixes based on region:
// us-*     → us.{model}
// eu-*     → eu.{model}
// ap-*     → apac.{model}, jp.{model}, or au.{model}
// global.* → unchanged
```

Region detection chain:
1. `providerConfig.options.region`
2. `AWS_REGION` env var
3. Default: `us-east-1`

### OpenAI
```typescript
{
  autoload: false,
  getModel(sdk, modelID) {
    return sdk.responses(modelID)  // Uses responses API, not chat
  }
}
```

### GitHub Copilot
```typescript
{
  autoload: false,
  getModel(sdk, modelID) {
    if (modelID.includes("codex")) {
      return sdk.responses(modelID)
    }
    return sdk.chat(modelID)
  }
}
```

---

## State Initialization Flow

```
1. Load Config (Config.get())
2. Fetch ModelsDev data (remote API with cache fallback)
3. Build database from ModelsDev → Provider.Info
4. Process config providers (merge with database)
5. Check environment variables for API keys
6. Check Auth store for credentials
7. Run custom loaders (autoload detection)
8. Apply allow/deny lists
9. Filter deprecated/alpha models
10. Cache SDK instances by hash
```

---

## Provider Resolution Priority

```
1. Environment Variables (source: "env")
   └─ Check Info.env array against process.env

2. Auth Store (source: "api")
   └─ Check Auth.get(providerID)

3. Plugin Authentication (source: "custom")
   └─ Plugin.list() → plugin.auth.loader()

4. Config File (source: "config")
   └─ Config.provider[providerID]
```

---

## Model Selection Algorithms

### Default Model
```typescript
priority = ["gpt-5", "claude-sonnet-4", "big-pickle", "gemini-3-pro"]

1. Get first available provider from config
2. Sort models by priority array
3. Prefer "latest" suffix
4. Return {providerID, modelID}
```

### Small Model Selection
```typescript
priority = [
  "claude-haiku-4-5",
  "gemini-3-flash",
  "gemini-2.5-flash",
  "gpt-5-nano"
]

// Provider-specific overrides:
// opencode: ["gpt-5-nano"]
// github-copilot: ["gpt-5-mini", "claude-haiku-4.5", ...]
```

---

## SDK Caching

SDKs are cached by configuration hash:

```typescript
const key = Bun.hash.xxHash32(JSON.stringify({
  npm: model.api.npm,
  options
}))
const existing = s.sdk.get(key)
if (existing) return existing
```

---

## Fetch Wrapper

All provider SDKs use a custom fetch wrapper:

```typescript
options["fetch"] = async (input, init) => {
  const opts = init ?? {}

  // Timeout handling
  if (options["timeout"]) {
    opts.signal = AbortSignal.any([
      opts.signal,
      AbortSignal.timeout(options["timeout"])
    ])
  }

  // OpenAI ID sanitization
  if (model.api.npm === "@ai-sdk/openai") {
    // Remove 'id' from input items unless Azure store=true
  }

  return fetch(input, { ...opts, timeout: false })
}
```

---

## Error Types

```typescript
ModelNotFoundError = NamedError.create("ProviderModelNotFoundError", z.object({
  providerID: z.string(),
  modelID: z.string(),
  suggestions: z.array(z.string()).optional(),
}))

InitError = NamedError.create("ProviderInitError", z.object({
  providerID: z.string(),
}))
```

Fuzzy matching provides suggestions on model not found:
```typescript
const matches = fuzzysort.go(modelID, availableModels, {
  limit: 3,
  threshold: -10000
})
```

---

## Transform Pipeline

### Message Normalization

```typescript
ProviderTransform.message(msgs, model, options)

1. unsupportedParts()     - Replace unsupported modalities with error text
2. normalizeMessages()    - Provider-specific message formatting
3. applyCaching()         - Add cache control headers (Anthropic/Bedrock)
4. Remap providerOptions  - Convert stored keys to SDK-expected keys
```

### Provider-Specific Normalizations

**Anthropic**: Filter empty messages, sanitize tool call IDs (alphanumeric + `_-`)

**Mistral**:
- Sanitize tool IDs to exactly 9 alphanumeric characters
- Insert filler assistant message between tool→user transitions

**Interleaved Reasoning**: Extract reasoning parts into `providerOptions.openaiCompatible.reasoning_content`

### Cache Control Application

```typescript
// Applied to first 2 system messages + last 2 messages
const providerOptions = {
  anthropic: { cacheControl: { type: "ephemeral" } },
  openrouter: { cacheControl: { type: "ephemeral" } },
  bedrock: { cachePoint: { type: "ephemeral" } },
  openaiCompatible: { cache_control: { type: "ephemeral" } },
}
```

---

## Reasoning Variants

Models with `capabilities.reasoning=true` get variant configurations:

### OpenAI/Azure
```typescript
efforts = ["none", "minimal", "low", "medium", "high", "xhigh"]
variant = {
  reasoningEffort: effort,
  reasoningSummary: "auto",
  include: ["reasoning.encrypted_content"]
}
```

### Anthropic
```typescript
variants = {
  high: { thinking: { type: "enabled", budgetTokens: 16000 } },
  max: { thinking: { type: "enabled", budgetTokens: 31999 } }
}
```

### Amazon Bedrock (Anthropic)
```typescript
variants = {
  high: { reasoningConfig: { type: "enabled", budgetTokens: 16000 } },
  max: { reasoningConfig: { type: "enabled", budgetTokens: 31999 } }
}
```

### Google
```typescript
// gemini-2.5
variants = {
  high: { thinkingConfig: { includeThoughts: true, thinkingBudget: 16000 } },
  max: { thinkingConfig: { includeThoughts: true, thinkingBudget: 24576 } }
}

// gemini-3
variants = {
  low: { includeThoughts: true, thinkingLevel: "low" },
  high: { includeThoughts: true, thinkingLevel: "high" }
}
```

---

## Temperature Defaults

```typescript
ProviderTransform.temperature(model):
  qwen     → 0.55
  claude   → undefined (provider default)
  gemini   → 1.0
  glm-4.6  → 1.0
  glm-4.7  → 1.0
  minimax  → 1.0
  kimi-k2  → 0.6 (1.0 if thinking variant)
  default  → undefined
```

---

## Models.dev Integration

External model database at `models.dev` API:

```typescript
// Fetch chain:
1. Read cached ${cache_path}/models.json
2. Fallback to macro-generated ModelsMacro.models
3. Background refresh every 60 minutes
4. 10-second fetch timeout
```

### Provider Schema from ModelsDev

```typescript
ModelsDev.Provider = {
  id: string,
  api?: string,       // Base URL
  name: string,
  env?: string[],     // Environment variables
  npm?: string,       // SDK package
  models: Record<string, Model>
}
```

---

## Key Constants

```typescript
// Model status filtering
VALID_STATUSES = ["active", "beta"]  // alpha excluded without flag
FILTER_ALPHA = !Flag.OPENCODE_ENABLE_EXPERIMENTAL_MODELS

// Default model priority
PRIORITY = ["gpt-5", "claude-sonnet-4", "big-pickle", "gemini-3-pro"]

// Small model priority
SMALL_PRIORITY = ["claude-haiku-4-5", "gemini-3-flash", "gpt-5-nano"]

// GitHub Copilot inheritance
github-copilot-enterprise inherits from github-copilot
```

---

## Configuration Interface

```yaml
# opencode.yaml
provider:
  openai:
    env: [OPENAI_API_KEY]
    options:
      timeout: 30000
    models:
      gpt-5:
        name: "GPT-5 Custom"
        options:
          temperature: 0.7
        variants:
          high:
            disabled: true
    blacklist: [gpt-4-turbo]
    whitelist: [gpt-5, gpt-5-mini]

disabled_providers: [mistral, cohere]
enabled_providers: [openai, anthropic]  # If set, only these are allowed

model: "openai/gpt-5"       # Default model
small_model: "openai/gpt-5-nano"
```

---

## Public API

```typescript
Provider.list(): Promise<Record<string, Info>>
Provider.getProvider(providerID): Promise<Info | undefined>
Provider.getModel(providerID, modelID): Promise<Model>
Provider.getLanguage(model): Promise<LanguageModelV2>
Provider.getSmallModel(providerID): Promise<Model | undefined>
Provider.defaultModel(): Promise<{providerID, modelID}>
Provider.parseModel("provider/model"): {providerID, modelID}
Provider.sort(models): Model[]
Provider.closest(providerID, query[]): {providerID, modelID} | undefined
```

---

## BRUTUS Implementation Notes

### Required Components

1. **Model Registry**: Store model metadata (capabilities, costs, limits)
2. **Provider Registry**: Map provider IDs to SDK factory functions
3. **SDK Cache**: Reuse SDK instances by configuration hash
4. **Transform Pipeline**: Message normalization + caching headers
5. **Auth Integration**: Support env vars and stored credentials

### Simplified Approach

For Go implementation, consider:

```go
type Model struct {
    ID         string
    ProviderID string
    APIID      string
    BaseURL    string
    Name       string
    Capabilities Capabilities
    Cost       Cost
    Limits     Limits
}

type Provider struct {
    ID      string
    Name    string
    EnvVars []string
    Models  map[string]Model
    Client  *openai.Client  // OpenAI-compatible client
}
```

### Critical Path

1. Load model database (embed JSON or fetch from models.dev)
2. Detect available providers from environment
3. Create OpenAI-compatible client per provider
4. Transform requests per provider requirements
5. Handle reasoning variants as model options
