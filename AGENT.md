# Agent Instructions

## Development Environment
This project uses [devenv](https://devenv.sh/) for reproducible development environments with Nix.

## Commands
- `devenv shell` - Enter the development shell
- `devenv test` - Run tests (currently runs git version check)
- `go build` - Build Go project
- `go run main.go` - Run the chat application
- `go test ./...` - Run all Go tests
- `go test <package>` - Run tests for specific package
- `go mod tidy` - Download dependencies
- `hello` - Custom script that greets from the development environment

### Application Commands
- `go run chat.go` - Simple chat interface with Claude
- `go run read.go` - Chat with file reading capabilities
- `go run list_files.go` - Chat with file listing and reading capabilities
- `go run bash_tool.go` - Chat with file operations and bash command execution
- `go run edit_tool.go` - Chat with full file operations (read, list, edit, bash)

### Verbose Logging
All Go applications support a `--verbose` flag for detailed execution logging:
- `go run chat.go --verbose` - Enable verbose logging for debugging
- `go run read.go --verbose` - See detailed tool execution and API calls
- `go run edit_tool.go --verbose` - Debug file operations and tool usage

## Architecture
- **Environment**: Nix-based development environment using devenv
- **Shell**: Includes Git, Go toolchain, and custom greeting script
- **Structure**: Chat application with terminal interface to Claude via Anthropic API

## Code Style Guidelines
- Follow Nix conventions for devenv.nix configuration
- Use standard Git workflows
- Development environment configuration should be reproducible

## Troubleshooting

### Verbose Logging
When debugging issues with the chat applications, use the `--verbose` flag to get detailed execution logs:

```bash
go run edit_tool.go --verbose
```

**What verbose logging shows:**
- API calls to Claude (model, timing, success/failure)
- Tool execution details (which tools are called, input parameters, results)
- File operations (reading, writing, listing files with sizes/counts)
- Bash command execution (commands run, output, errors)
- Conversation flow (message processing, content blocks)
- Error details with stack traces

**Log output locations:**
- **Verbose mode**: Detailed logs go to stderr with timestamps and file locations
- **Normal mode**: Only essential output goes to stdout

**Common troubleshooting scenarios:**
- **API failures**: Check verbose logs for authentication errors or rate limits
- **Tool failures**: See exactly which tool failed and why (file not found, permission errors)
- **Unexpected responses**: View full conversation flow and Claude's reasoning
- **Performance issues**: See API call timing and response sizes

### Environment Issues
- Ensure `ANTHROPIC_API_KEY` environment variable is set
- Run `devenv shell` to ensure proper development environment
- Use `go mod tidy` to ensure dependencies are installed

## Notes
- Requires ANTHROPIC_API_KEY environment variable to be set
- Chat application provides a simple terminal interface to Claude
- Use ctrl-c to quit the chat session
