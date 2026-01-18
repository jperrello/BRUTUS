# OpenCode Provider Research

**Researcher**: Claude Agent
**Date**: 2026-01-17
**Subject**: Provider management subsystem reverse engineering

---

## What Was Researched

Deep analysis of OpenCode's provider subsystem located at `packages/opencode/src/provider/`. This manages connections to 20+ LLM providers through a unified interface.

## Files in This Directory

| File | Description |
|------|-------------|
| `PROVIDER-SUBSYSTEM-SPEC.md` | Complete specification - architecture, data structures, custom loaders, transform pipeline |

## Key Findings

### Architecture
- Lazy initialization with instance-based singleton state
- SDK caching via xxHash32 of configuration
- Custom fetch wrappers for timeout + request modification
- Fuzzy matching for model not found suggestions

### Bundled Providers (21 total)
```
@ai-sdk/amazon-bedrock     @ai-sdk/anthropic
@ai-sdk/azure              @ai-sdk/google
@ai-sdk/google-vertex      @ai-sdk/openai
@openrouter/ai-sdk-provider @ai-sdk/xai
@ai-sdk/mistral            @ai-sdk/groq
@ai-sdk/deepinfra          @ai-sdk/cerebras
@ai-sdk/cohere             @ai-sdk/gateway
@ai-sdk/togetherai         @ai-sdk/perplexity
@ai-sdk/vercel             @gitlab/gitlab-ai-provider
@ai-sdk/openai-compatible  @ai-sdk/github-copilot (custom)
```

### Provider Resolution Priority
1. Environment variables (highest)
2. Auth store (stored credentials)
3. Plugin authentication
4. Config file (lowest)

### Key Transforms
- **Anthropic**: Empty message filtering, tool ID sanitization
- **Mistral**: 9-char alphanumeric tool IDs, filler messages
- **Bedrock**: Region-based model ID prefixes (us., eu., apac., jp., au.)
- **Caching**: Ephemeral cache headers on system + recent messages

### Reasoning Variants
Models with reasoning capability get effort variants:
- OpenAI: none, minimal, low, medium, high, xhigh
- Anthropic: high (16K tokens), max (32K tokens)
- Google: thinkingLevel or thinkingBudget based on model version

## What Was NOT Researched

- Agent loop integration
- Session management
- Tool definitions
- UI components
- Other packages (console, desktop, etc.)

## Next Steps for Future Agents

1. **Implement Saturn adapter**: Map Saturn discovery to provider interface
2. **Model metadata**: Embed or fetch model database
3. **Transform layer**: Port message normalization to Go
4. **Reasoning support**: Implement variant selection
5. **Caching strategy**: Add ephemeral cache headers for supported providers

## Source Files Analyzed

- `packages/opencode/src/provider/provider.ts` (main)
- `packages/opencode/src/provider/models.ts` (ModelsDev integration)
- `packages/opencode/src/provider/transform.ts` (message transforms)
- `packages/opencode/src/provider/auth.ts` (provider auth)
- `packages/opencode/src/provider/sdk/openai-compatible/src/` (custom SDK)
