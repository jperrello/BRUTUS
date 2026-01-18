# OpenCode Server-Sent Events Protocol

## Overview

OpenCode uses Server-Sent Events (SSE) for real-time push notifications from server to client. This is the primary mechanism for:
- Session state updates
- Message streaming during inference
- Tool execution progress
- Permission requests
- System notifications

## Connection Endpoints

### Instance Events
```
GET /event
```
Streams events for the current instance (project directory).

### Global Events
```
GET /global/event
```
Streams events across all instances.

## Connection Protocol

### Establishing Connection

```http
GET /event HTTP/1.1
Host: localhost:4096
Accept: text/event-stream
Cache-Control: no-cache
X-Directory: /path/to/project
```

### Response Headers

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

## Event Format

Events follow the SSE standard format:

```
event: {event_type}
data: {json_payload}

```

Note the double newline after data.

## Event Types

### Connection Events

#### server.connected
Sent immediately upon connection establishment.

```
event: server.connected
data: {}
```

#### server.heartbeat
Sent every 30 seconds to maintain connection.

```
event: server.heartbeat
data: {}
```

#### global.disposed
Sent when all instances are disposed.

```
event: global.disposed
data: {}
```

### Session Events

Based on the bus event system, these events are forwarded to connected clients:

#### session.created
```json
{
  "sessionID": "abc123",
  "title": "New Session",
  "time": { "created": "2024-01-01T00:00:00Z" }
}
```

#### session.updated
```json
{
  "sessionID": "abc123",
  "title": "Updated Title"
}
```

#### session.deleted
```json
{
  "sessionID": "abc123"
}
```

### Message Events

#### message.created
New message added to session.

#### message.updated
Message content changed (e.g., streaming completion).

#### message.part.created
New part added to message (tool call, text block).

#### message.part.updated
Part content changed.

#### message.part.deleted
Part removed from message.

### Inference Events

#### inference.started
Agent began processing.

#### inference.completed
Agent finished processing.

#### inference.error
Error during inference.

### Permission Events

#### permission.requested
Agent is requesting permission for an action.

```json
{
  "requestID": "perm123",
  "sessionID": "sess456",
  "tool": "bash",
  "input": { "command": "rm -rf node_modules" },
  "description": "Execute shell command"
}
```

#### permission.responded
User responded to permission request.

### Tool Events

#### tool.started
Tool execution began.

#### tool.completed
Tool execution finished.

#### tool.error
Tool execution failed.

### File Events

#### file.changed
File was modified.

#### file.created
File was created.

#### file.deleted
File was deleted.

## Event Payload Structure

All events include the instance directory for routing:

```json
{
  "directory": "/path/to/project",
  "payload": {
    // Event-specific data
  }
}
```

## Heartbeat Mechanism

The server sends a heartbeat every 30 seconds:

```javascript
const HEARTBEAT_INTERVAL = 30000 // 30 seconds

setInterval(() => {
  stream.writeSSE({
    event: "server.heartbeat",
    data: "{}"
  })
}, HEARTBEAT_INTERVAL)
```

Clients should:
1. Expect heartbeats every 30 seconds
2. Consider connection dead if no heartbeat for 60+ seconds
3. Implement automatic reconnection

## Client Implementation

### JavaScript Example

```javascript
const eventSource = new EventSource(
  'http://localhost:4096/event?directory=/path/to/project'
)

eventSource.addEventListener('server.connected', (e) => {
  console.log('Connected to OpenCode server')
})

eventSource.addEventListener('server.heartbeat', (e) => {
  // Connection is alive
})

eventSource.addEventListener('message.updated', (e) => {
  const data = JSON.parse(e.data)
  // Update UI with new message content
})

eventSource.addEventListener('permission.requested', (e) => {
  const data = JSON.parse(e.data)
  // Show permission dialog
})

eventSource.onerror = (e) => {
  console.error('SSE connection error', e)
  // Implement reconnection logic
}
```

### Go Example

```go
req, _ := http.NewRequest("GET", "http://localhost:4096/event", nil)
req.Header.Set("Accept", "text/event-stream")
req.Header.Set("X-Directory", "/path/to/project")

resp, _ := http.DefaultClient.Do(req)
reader := bufio.NewReader(resp.Body)

for {
    line, _ := reader.ReadString('\n')
    if strings.HasPrefix(line, "event:") {
        eventType := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
        // Handle event type
    } else if strings.HasPrefix(line, "data:") {
        data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
        // Parse JSON data
    }
}
```

## Reconnection Strategy

Recommended reconnection approach:

```javascript
class OpenCodeSSE {
  constructor(baseUrl, directory) {
    this.baseUrl = baseUrl
    this.directory = directory
    this.reconnectDelay = 1000
    this.maxReconnectDelay = 30000
    this.connect()
  }

  connect() {
    this.eventSource = new EventSource(
      `${this.baseUrl}/event?directory=${encodeURIComponent(this.directory)}`
    )

    this.eventSource.onopen = () => {
      this.reconnectDelay = 1000 // Reset on successful connection
    }

    this.eventSource.onerror = () => {
      this.eventSource.close()
      setTimeout(() => this.connect(), this.reconnectDelay)
      this.reconnectDelay = Math.min(
        this.reconnectDelay * 2,
        this.maxReconnectDelay
      )
    }
  }
}
```

## Event Bus Integration

The server subscribes to the internal bus and forwards events:

```typescript
// Simplified server event forwarding
const bus = Bus.subscribe()

for await (const event of bus) {
  stream.writeSSE({
    event: event.type,
    data: JSON.stringify({
      directory: instance.directory,
      payload: event.payload
    })
  })
}
```

## Cleanup

When a client disconnects:
1. SSE connection is closed
2. Bus subscription is cleaned up
3. Server logs disconnection
4. No explicit cleanup message sent to other clients
