# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Ritual** is a CLI-based workflow engine written in Go that executes declarative YAML workflows called "incantations". The project implements task dependencies, templating with Sprig functions, and native task types.

## Development Commands

Since this is a new Go project, typical development commands will be:

```bash
# Initialize Go module (if not already done)
go mod init github.com/sarlalian/ritual

# Build the project
go build -o ritual ./cmd/ritual

# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run a specific test
go test -run TestName ./path/to/package

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Tidy dependencies
go mod tidy

# Install tools (once implemented)
go install ./cmd/ritual
```

The project may implement a Justfile for common development tasks as outlined in the plan.

## Architecture Overview

The project follows a modular architecture with these core components:

### 1. CLI Application
- Built using Cobra framework
- Main entry point with subcommands for different execution modes
- Configuration loading with Viper

### 2. Workflow Engine Core
- **Workflow Parser**: YAML parsing and validation for incantation files
- **Template Engine**: text/template integration with Sprig functions
- **Task Engine**: Execution framework with dependency resolution using topological sort
- **Dependency Resolver**: Handles task dependencies and execution order

### 3. Native Task Types
The system implements a common Task interface with these built-in task types:
- **Command Task**: Execute shell commands with environment variable support
- **SSH Task**: Execute commands over SSH connections
- **Compress/Decompress**: Support for Bzip2, LZMA2, Gzip, Zip formats
- **Checksum/Validate**: SHA256, SHA512, Blake, MD5 hash operations
- **Email**: SMTP and AWS SES email sending
- **Slack**: Post messages to Slack channels
- **S3 Operations**: Upload/download with tags and ACL support

### 4. Filesystem Abstraction
- Uses Afero for filesystem operations
- Supports Local, HTTP, S3, GcsFs, Zip, Tar, DriveFs backends

### 5. Output & Event System
- **Output Handler**: JSON results to file:// or s3:// destinations
- **Reporting Engine**: Status aggregation and reporting via email/slack/terminal
- **Event Orchestration**: Google Pub/Sub integration for workflow triggering

### 6. Execution Modes
- Direct workflow execution
- HTTP server mode for webhook-triggered workflows
- Google Pub/Sub listener for event-driven execution
- File system watcher for file-based triggers

## Key Design Patterns

### Task Interface
All task types implement a common interface enabling polymorphic execution and consistent error handling.

### Dependency Graph
- Tasks declare dependencies via YAML configuration
- Topological sort determines execution order
- Circular dependency detection prevents infinite loops

### Template System
- All strings in workflows support Go template syntax with Sprig functions
- Templates resolved once at workflow start for consistency
- Built-in variables like hostname, timestamps available

### Error Handling
- Distinction between required and optional tasks
- Configurable retry logic with exponential backoff
- Structured logging with task-specific context
- Comprehensive error propagation

### Modular Output
- JSON-formatted execution results
- Pluggable output destinations (file, S3)
- Task status tracking and workflow-level aggregation

## Implementation Phases

The project follows an incremental development approach across 5 phases:

1. **Foundation**: Project setup, core interfaces, CLI skeleton, test framework
2. **Workflow Engine Core**: YAML parsing, dependency resolution, task execution
3. **Native Tasks**: Implementation of all built-in task types
4. **Output & Error Handling**: JSON formatting, retry logic, error management
5. **Integration & Polish**: End-to-end testing, documentation, examples

## File Organization

Expected project structure:
```
cmd/ritual/          # Main CLI application
internal/
  workflow/          # Workflow parsing and execution
  tasks/             # Native task implementations
  template/          # Template engine
  output/            # Output handlers
  events/            # Event orchestration
pkg/                 # Public API packages
examples/            # Example incantation files
docs/                # Documentation
```

## Testing Strategy

- **Unit Tests**: Each component tested in isolation
- **Integration Tests**: Full workflow execution scenarios
- **Example Workflows**: Real-world usage validation
- **Error Cases**: Comprehensive failure mode testing
- **Performance**: Benchmark critical execution paths

## Development Notes

- Use structured logging for all components
- Implement proper context cancellation for long-running operations
- All configuration should support environment variable overrides
- Template evaluation should be fail-fast with clear error messages
- Task implementations should be stateless and thread-safe
