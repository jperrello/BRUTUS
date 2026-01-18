# OpenCode HTTP API Specification

## Base URL

```
http://localhost:4096
```

## Authentication

Optional basic authentication via environment configuration. When enabled, all requests require standard HTTP Basic Auth header.

## Instance Routing

All requests must specify the project directory via:
- Query parameter: `?directory=/path/to/project`
- Header: `X-Directory: /path/to/project`

---

## Core Endpoints

### GET /doc
Returns OpenAPI 3.1.1 specification document.

### GET /path
Returns filesystem paths for the current instance.

**Response:**
```json
{
  "home": "/home/user",
  "state": "/home/user/.local/state/opencode",
  "config": "/home/user/.config/opencode",
  "worktree": "/path/to/project",
  "directory": "/path/to/project"
}
```

### GET /vcs
Returns version control status.

**Response:** `Vcs.Info` object with branch information.

### GET /command
Lists all available slash commands.

**Response:** `Command.Info[]`

### GET /agent
Lists available AI agents (build, plan, etc.).

**Response:** `Agent.Info[]`

### GET /skill
Lists available skills/capabilities.

**Response:** `Skill.Info[]`

### GET /lsp
Returns Language Server Protocol status.

**Response:** `LSP.Status`

### GET /formatter
Returns code formatter status.

**Response:** `Format.Status`

### POST /log
Accepts client-side log entries.

**Request:**
```json
{
  "service": "tui",
  "level": "info",
  "message": "User clicked button",
  "extra": { "buttonId": "submit" }
}
```

### PUT /auth/:providerID
Sets authentication credentials for a provider.

**Request:** `Auth.Info` schema

### GET /event
Server-Sent Events stream (see SSE-EVENT-PROTOCOL.md).

### POST /instance/dispose
Disposes the current instance, releasing all resources.

**Response:** `boolean`

---

## Global Routes (`/global`)

### GET /global/health
Health check endpoint.

**Response:**
```json
{
  "healthy": true,
  "version": "1.2.3"
}
```

### GET /global/event
SSE stream for global events across all instances.

### POST /global/dispose
Disposes ALL instances.

**Response:** `boolean`

---

## Project Routes (`/project`)

### GET /project/projects
Lists all projects opened with OpenCode.

**Response:** `Project.Info[]`

### GET /project/projects/current
Returns the currently active project.

**Response:** `Project.Info`

### PATCH /project/projects/:projectID
Updates project attributes.

**Request:**
```json
{
  "name": "My Project",
  "icon": "folder",
  "color": "#ff5500"
}
```

**Response:** `Project.Info`

---

## Session Routes (`/session`)

### GET /session
Lists all sessions.

**Query:**
- `directory`: Filter by directory
- `roots`: Boolean, only root sessions
- `start`: Timestamp filter
- `search`: Text search
- `limit`: Max results

**Response:** `Session.Info[]`

### GET /session/status
Returns session subsystem status.

### POST /session
Creates a new session.

**Request:** `Session.create.schema` (optional)

**Response:** `Session.Info`

### GET /session/:sessionID
Returns session details.

**Response:** `Session.Info`

### DELETE /session/:sessionID
Deletes a session.

### PATCH /session/:sessionID
Updates session attributes.

**Request:**
```json
{
  "title": "New Title",
  "time": { "archived": "2024-01-01T00:00:00Z" }
}
```

### GET /session/:sessionID/children
Lists child sessions (forked from this session).

**Response:** `Session.Info[]`

### GET /session/:sessionID/todo
Returns todo items from the session.

### POST /session/:sessionID/init
Initializes session with system prompt and configuration.

**Request:** `Session.initialize.schema` (minus sessionID)

### POST /session/:sessionID/fork
Creates a new session branched from this one.

**Request:** `Session.fork.schema` (minus sessionID)

### POST /session/:sessionID/abort
Cancels the currently running inference.

### POST /session/:sessionID/share
Creates a shareable link for the session.

### DELETE /session/:sessionID/share
Removes sharing for the session.

### GET /session/:sessionID/diff
Returns file diffs for the session.

**Query:** `messageID` (optional, filter to specific message)

### POST /session/:sessionID/summarize
Generates a summary of the session.

**Request:**
```json
{
  "providerID": "anthropic",
  "modelID": "claude-3-haiku",
  "auto": true
}
```

### GET /session/:sessionID/message
Lists messages in the session.

**Query:** `limit` (number)

**Response:** `MessageV2.WithParts[]`

### GET /session/:sessionID/message/:messageID
Returns a specific message.

### DELETE /session/:sessionID/message/:messageID/part/:partID
Deletes a message part (e.g., a tool call).

### PATCH /session/:sessionID/message/:messageID/part/:partID
Updates a message part.

**Request:** `MessageV2.Part` schema

### POST /session/:sessionID/message
Sends a user prompt.

**Request:** `SessionPrompt.PromptInput` (minus sessionID)

### POST /session/:sessionID/prompt_async
Sends prompt without waiting for response.

### POST /session/:sessionID/command
Executes a slash command.

**Request:** `SessionPrompt.CommandInput` (minus sessionID)

### POST /session/:sessionID/shell
Executes a shell command in session context.

**Request:** `SessionPrompt.ShellInput` (minus sessionID)

### POST /session/:sessionID/revert
Reverts session to a previous state.

**Request:** `SessionRevert.RevertInput` (minus sessionID)

### POST /session/:sessionID/unrevert
Undoes a revert operation.

### POST /session/:sessionID/permissions/:permissionID
Responds to a permission request within the session.

**Request:**
```json
{
  "response": "allow"
}
```

---

## PTY Routes (`/pty`)

### GET /pty
Lists all PTY (terminal) sessions.

**Response:** `Pty.Info[]`

### POST /pty
Creates a new PTY session.

**Request:** `Pty.create.schema`

**Response:** `Pty.Info`

### GET /pty/:ptyID
Returns PTY session details.

### PUT /pty/:ptyID
Updates PTY session (e.g., resize).

**Request:** `Pty.update.schema`

### DELETE /pty/:ptyID
Terminates a PTY session.

**Response:** `boolean`

### GET /pty/:ptyID/connect
Upgrades to WebSocket for terminal I/O.

**Protocol:** WebSocket
- Bidirectional binary/text streaming
- Handles open, message, close events

---

## File Routes (`/`)

### GET /find
Searches file contents with ripgrep.

**Query:** `pattern` (regex)

**Response:** `Ripgrep.Match[]`

### GET /find/file
Searches for files by name.

**Query:**
- `query`: Search pattern
- `dirs`: Include directories (boolean)
- `type`: "file" or "directory"
- `limit`: 1-200

**Response:** `string[]` (paths)

### GET /find/symbol
Searches for code symbols via LSP.

**Query:** `query`

**Response:** `LSP.Symbol[]`

### GET /file
Lists directory contents.

**Query:** `path`

**Response:** `File.Node[]`

### GET /file/content
Reads file contents.

**Query:** `path`

**Response:** `File.Content`

### GET /file/status
Returns git status of all files.

**Response:** `File.Info[]`

---

## Config Routes (`/config`)

### GET /config/config
Returns current configuration.

**Response:** `Config.Info`

### PATCH /config/config
Updates configuration.

**Request:** `Config.Info`

### GET /config/config/providers
Lists configured AI providers.

**Response:**
```json
{
  "providers": [...],
  "default": { "anthropic": "claude-3-opus", ... }
}
```

---

## Provider Routes (`/provider`)

### GET /provider
Lists all providers.

**Response:**
```json
{
  "all": [...],
  "default": { ... },
  "connected": ["anthropic", "openai"]
}
```

### GET /provider/auth
Returns authentication methods per provider.

**Response:** `Record<string, AuthMethod[]>`

### POST /provider/:providerID/oauth/authorize
Initiates OAuth flow.

**Request:** `{ "method": 0 }`

**Response:** `ProviderAuth.Authorization | null`

### POST /provider/:providerID/oauth/callback
Completes OAuth with authorization code.

**Request:** `{ "method": 0, "code": "abc123" }`

**Response:** `boolean`

---

## MCP Routes (`/mcp`)

### GET /mcp
Returns status of all MCP servers.

**Response:** `MCP.Status[]`

### POST /mcp
Adds a new MCP server.

**Request:**
```json
{
  "name": "my-server",
  "config": { ... }
}
```

### POST /mcp/:name/auth
Initiates OAuth for MCP server.

### POST /mcp/:name/auth/callback
Completes OAuth with code.

**Request:** `{ "code": "abc123" }`

### POST /mcp/:name/auth/authenticate
Starts OAuth and waits for browser callback.

### DELETE /mcp/:name/auth
Removes OAuth credentials.

### POST /mcp/:name/connect
Connects to MCP server.

**Response:** `boolean`

### POST /mcp/:name/disconnect
Disconnects from MCP server.

**Response:** `boolean`

---

## Permission Routes (`/permission`)

### GET /permission
Lists pending permission requests.

**Response:** `PermissionNext.Request[]`

### POST /permission/:requestID/reply
Responds to a permission request.

**Request:**
```json
{
  "reply": "allow",
  "message": "Approved for this session"
}
```

**Response:** `boolean`

---

## Question Routes (`/question`)

### GET /question/questions
Lists pending questions.

**Response:** `Question.Request[]`

### POST /question/questions/:requestID/reply
Answers a question.

**Request:** `Question.Reply` (with answers)

**Response:** `boolean`

### POST /question/questions/:requestID/reject
Rejects a question.

**Response:** `boolean`

---

## TUI Routes (`/tui`)

These endpoints allow programmatic control of the terminal UI.

### POST /tui/append-prompt
Appends text to the prompt input.

### POST /tui/open-help
Opens help dialog.

### POST /tui/open-sessions
Opens sessions dialog.

### POST /tui/open-themes
Opens themes dialog.

### POST /tui/open-models
Opens model selector.

### POST /tui/submit-prompt
Submits the current prompt.

### POST /tui/clear-prompt
Clears prompt input.

### POST /tui/execute-command
Executes a command.

### POST /tui/show-toast
Shows a toast notification.

### POST /tui/publish
Emits a typed TUI event.

### POST /tui/select-session
Navigates to a session.

### GET /tui/control/next
Retrieves next queued TUI request.

### POST /tui/control/response
Submits response to TUI request.

---

## Experimental Routes (`/experimental`)

### GET /experimental/tool/ids
Lists all tool IDs.

**Response:** `string[]`

### GET /experimental/tool
Lists tools with details.

**Query:** `provider`, `model`

**Response:**
```json
[
  {
    "id": "read_file",
    "description": "Read file contents",
    "parameters": { ... }
  }
]
```

### POST /experimental/worktree
Creates a git worktree.

**Request:** `Worktree.create.schema`

**Response:** `Worktree.Info`

### GET /experimental/worktree
Lists sandbox worktrees.

**Response:** `string[]`

### GET /experimental/resource
Gets MCP resources.

**Response:** `Record<string, MCP.Resource>`
