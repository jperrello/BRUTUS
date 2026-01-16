---
description: Debug BRUTUS when something is broken
allowed-tools: [Read, Glob, Grep, Bash, TodoWrite]
---

# BRUTUS Debug Agent

Something's broken. Let's diagnose systematically.

## Diagnostic Steps

### 1. Build Check
```bash
go build -o brutus.exe . 2>&1
```
If this fails, the error message tells you exactly what's wrong.

### 2. Vet Check
```bash
go vet ./... 2>&1
```
Catches issues the compiler misses.

### 3. Dependency Check
```bash
go mod tidy
go mod verify
```

### 4. If Runtime Error

Check these in order:

**Saturn Discovery Issues:**
- Is a Saturn server running? (`dns-sd -B _saturn._tcp local.`)
- Check `provider/discovery.go` for parsing issues
- Check `provider/saturn.go` for API issues

**Tool Execution Issues:**
- Which tool failed? Check `tools/[toolname].go`
- Is input parsing correct? Check the Input struct
- Is the tool registered in `main.go`?

**Agent Loop Issues:**
- Check `agent/agent.go` - the `Run()` function
- Is conversation history being built correctly?
- Are tool results being sent back properly?

### 5. Trace the Code Path

For the specific issue:
1. Identify the entry point
2. Follow the execution path
3. Find where it diverges from expected behavior

## Common Issues

| Symptom | Likely Cause | Check |
|---------|--------------|-------|
| "no saturn services" | No server on network | Saturn server running? |
| Build fails | Type error | Read the error message |
| Tool not found | Not registered | `main.go` registry |
| JSON unmarshal error | Bad tool input | Tool's Input struct |
| Empty response | API issue | `provider/saturn.go` |

## Output

```markdown
## Diagnosis
[What's actually wrong]

## Root Cause
[Why it's happening]

## Fix
[Specific code changes needed]

## Verification
[How to confirm it's fixed]
```

---

**Error/Issue description:** $ARGUMENTS
