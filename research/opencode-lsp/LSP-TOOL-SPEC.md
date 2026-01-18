# OpenCode LSP Tool Specification

## Overview

OpenCode exposes LSP capabilities to the AI agent through a single unified tool called `lsp`. This tool provides code intelligence features that help the agent understand and navigate codebases.

## Tool Definition

```typescript
export const LspTool = Tool.define("lsp", {
  description: DESCRIPTION,
  parameters: z.object({
    operation: z.enum(operations),
    filePath: z.string(),
    line: z.number().int().min(1),
    character: z.number().int().min(1)
  }),
  execute: async (args, ctx) => { ... }
})
```

## Supported Operations

| Operation | LSP Method | Description |
|-----------|------------|-------------|
| goToDefinition | textDocument/definition | Find where a symbol is defined |
| findReferences | textDocument/references | Find all usages of a symbol |
| hover | textDocument/hover | Get documentation/type info |
| documentSymbol | textDocument/documentSymbol | List all symbols in a file |
| workspaceSymbol | workspace/symbol | Search symbols across workspace |
| goToImplementation | textDocument/implementation | Find interface implementations |
| prepareCallHierarchy | textDocument/prepareCallHierarchy | Get call hierarchy item at position |
| incomingCalls | callHierarchy/incomingCalls | Find callers of a function |
| outgoingCalls | callHierarchy/outgoingCalls | Find callees of a function |

## Input Schema

```typescript
{
  operation: "goToDefinition" | "findReferences" | "hover" |
             "documentSymbol" | "workspaceSymbol" |
             "goToImplementation" | "prepareCallHierarchy" |
             "incomingCalls" | "outgoingCalls",

  filePath: string,    // Absolute or relative path
  line: number,        // 1-based (matches editor display)
  character: number    // 1-based (matches editor display)
}
```

## Coordinate System

**Critical Detail**: The tool uses **1-based** indexing to match what editors display, but internally converts to **0-based** for the LSP protocol:

```typescript
const position = {
  file,
  line: args.line - 1,        // Convert to 0-based
  character: args.character - 1
}
```

This means when the agent sees "error on line 42, column 10" in a file, it can directly use those numbers without conversion.

## Execution Flow

```
┌────────────────────────────────────────────────────────────────┐
│                     LspTool.execute()                         │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ 1. Path Resolution                                             │
│    - If relative: path.join(Instance.directory, filePath)      │
│    - Convert to file:// URI for LSP                           │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ 2. Permission Check                                            │
│    await ctx.ask({                                             │
│      permission: "lsp",                                        │
│      patterns: ["*"],                                          │
│      always: ["*"]                                             │
│    })                                                          │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ 3. Validation                                                  │
│    - Check file exists: Bun.file(file).exists()                │
│    - Check LSP available: LSP.hasClients(file)                │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ 4. Touch File (ensures LSP server is ready)                    │
│    await LSP.touchFile(file, true)  // Wait for diagnostics   │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ 5. Execute Operation                                           │
│    switch (operation) {                                        │
│      case "goToDefinition": LSP.definition(position)           │
│      case "findReferences": LSP.references(position)           │
│      ...                                                       │
│    }                                                           │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────────┐
│ 6. Format Output                                               │
│    return {                                                    │
│      title: "goToDefinition src/main.ts:42:10",               │
│      metadata: { result },                                     │
│      output: JSON.stringify(result, null, 2)                  │
│    }                                                           │
└────────────────────────────────────────────────────────────────┘
```

## Output Formats

### goToDefinition / findReferences / goToImplementation

```json
[
  {
    "uri": "file:///path/to/file.ts",
    "range": {
      "start": { "line": 10, "character": 4 },
      "end": { "line": 10, "character": 20 }
    }
  }
]
```

### hover

```json
{
  "contents": {
    "kind": "markdown",
    "value": "```typescript\nfunction greet(name: string): string\n```\nGreets a person by name."
  },
  "range": {
    "start": { "line": 10, "character": 4 },
    "end": { "line": 10, "character": 9 }
  }
}
```

### documentSymbol

```json
[
  {
    "name": "MyClass",
    "kind": 5,
    "range": { "start": { "line": 0, "character": 0 }, "end": { "line": 50, "character": 1 } },
    "selectionRange": { "start": { "line": 0, "character": 6 }, "end": { "line": 0, "character": 13 } },
    "children": [
      {
        "name": "constructor",
        "kind": 9,
        "range": { "start": { "line": 2, "character": 2 }, "end": { "line": 5, "character": 3 } },
        "selectionRange": { "start": { "line": 2, "character": 2 }, "end": { "line": 2, "character": 13 } }
      }
    ]
  }
]
```

### workspaceSymbol

```json
[
  {
    "name": "MyClass",
    "kind": 5,
    "location": {
      "uri": "file:///path/to/file.ts",
      "range": { "start": { "line": 0, "character": 0 }, "end": { "line": 0, "character": 13 } }
    }
  }
]
```

### Call Hierarchy

```json
// prepareCallHierarchy
[
  {
    "name": "processData",
    "kind": 12,
    "uri": "file:///path/to/file.ts",
    "range": { "start": { "line": 10, "character": 0 }, "end": { "line": 20, "character": 1 } },
    "selectionRange": { "start": { "line": 10, "character": 9 }, "end": { "line": 10, "character": 20 } }
  }
]

// incomingCalls / outgoingCalls
[
  {
    "from": {
      "name": "handleRequest",
      "kind": 12,
      "uri": "file:///path/to/handler.ts"
    },
    "fromRanges": [
      { "start": { "line": 15, "character": 4 }, "end": { "line": 15, "character": 15 } }
    ]
  }
]
```

## Error Handling

| Condition | Error Message |
|-----------|---------------|
| File not found | "File not found: {path}" |
| No LSP server | "No LSP server available for this file type." |
| No results | "No results found for {operation}" |

## Tool Description (lsp.txt)

```
Interact with Language Server Protocol (LSP) servers to get code
intelligence features.

Supported Operations:
- goToDefinition: Find where a symbol is defined
- findReferences: Find all references to a symbol
- hover: Get hover information (documentation, type info) for a symbol
- documentSymbol: Get all symbols (functions, classes, variables) in a document
- workspaceSymbol: Search for symbols across the entire workspace
- goToImplementation: Find implementations of an interface or abstract method
- prepareCallHierarchy: Get call hierarchy item at a position (functions/methods)
- incomingCalls: Find all functions/methods that call the function at a position
- outgoingCalls: Find all functions/methods called by the function at a position

Required Parameters:
- filePath: The file to operate on
- line: The line number (1-based, as shown in editors)
- character: The character offset (1-based, as shown in editors)

Note: LSP servers must be configured for the file type; if no server
is available, an error will be returned.
```

## Usage Patterns for AI Agent

### Finding Definition

```json
{
  "operation": "goToDefinition",
  "filePath": "src/utils/helpers.ts",
  "line": 25,
  "character": 12
}
```

### Understanding a Function

```json
{
  "operation": "hover",
  "filePath": "src/api/client.ts",
  "line": 42,
  "character": 8
}
```

### Finding All Usages

```json
{
  "operation": "findReferences",
  "filePath": "src/types/index.ts",
  "line": 10,
  "character": 15
}
```

### Exploring File Structure

```json
{
  "operation": "documentSymbol",
  "filePath": "src/components/Button.tsx",
  "line": 1,
  "character": 1
}
```

### Understanding Call Graph

```json
// Step 1: Get the function at this position
{
  "operation": "prepareCallHierarchy",
  "filePath": "src/services/auth.ts",
  "line": 50,
  "character": 10
}

// Step 2: Who calls this function?
{
  "operation": "incomingCalls",
  "filePath": "src/services/auth.ts",
  "line": 50,
  "character": 10
}

// Step 3: What does this function call?
{
  "operation": "outgoingCalls",
  "filePath": "src/services/auth.ts",
  "line": 50,
  "character": 10
}
```
