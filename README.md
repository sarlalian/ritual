# 🔮 Ritual - Parallel Workflow Engine

A powerful CLI-based workflow engine written in Go that executes declarative YAML workflows ("incantations") with built-in parallel execution, dependency resolution, and templating capabilities.

## ✨ Features

- **⚡ Parallel Execution** - Execute independent tasks concurrently by default
- **🔗 Dependency Resolution** - Automatic topological sorting and execution layering
- **📝 Rich Templating** - Built-in Sprig functions for dynamic workflows
- **🎯 Multiple Task Types** - 7 built-in task types with 18 aliases
- **🔄 Context Sharing** - Share variables and task results between steps
- **🧪 Dry-Run Mode** - Preview workflows without making changes
- **📊 Structured Logging** - Comprehensive logging with multiple verbosity levels
- **🎨 Import System** - Compose workflows from multiple sources

## 🚀 Quick Start

### Installation

```bash
go build -o ritual ./cmd/ritual
```

### Basic Usage

```bash
# Execute a workflow
./ritual run workflow.yaml

# Validate workflow syntax
./ritual validate workflow.yaml

# Preview execution without running
./ritual dry-run workflow.yaml

# List available task types
./ritual list-tasks

# Get help
./ritual --help
```

## 📋 Task Types

### Core Tasks
- **command** / **shell** / **script** - Execute shell commands and scripts
- **file** / **copy** / **template** - File operations (create, copy, delete, chmod, etc.)

### Compression
- **compress** / **archive** / **unarchive** - Create and extract archives (tar, gzip, zip, bzip2)

### Security
- **checksum** / **hash** / **verify** - Calculate and verify file checksums (SHA256, SHA512, MD5, Blake2b)

### Remote Operations
- **ssh** / **remote** - Execute commands on remote hosts via SSH

### Communication
- **email** / **mail** - Send emails via SMTP with TLS support
- **slack** / **notify** - Post messages to Slack channels via webhooks

## 📝 Workflow Format

### Simple Example

```yaml
name: Simple Example Workflow
description: A basic workflow demonstrating core features
version: "1.0"

vars:
  project_name: "MyProject"
  timestamp: "{{ now | date \"2006-01-02\" }}"

tasks:
  - id: hello
    name: Say Hello
    type: command
    command: echo "Hello from {{ .vars.project_name }}!"

  - id: date
    name: Show Current Date
    type: command
    command: date
    depends_on: [hello]
```

### Advanced Example

```yaml
name: Advanced Workflow
description: Demonstrates parallel execution and dependencies
version: "1.0"

environment:
  ENVIRONMENT: "production"
  API_KEY: "${API_KEY}"

vars:
  temp_dir: "/tmp/ritual-demo"
  hostname: "{{ hostname }}"

tasks:
  # Create directory
  - id: setup
    name: Setup Workspace
    type: file
    path: "{{ .vars.temp_dir }}"
    state: directory
    mode: "0755"

  # Parallel tasks - both depend on setup
  - id: create_config
    name: Generate Config
    type: file
    path: "{{ .vars.temp_dir }}/config.json"
    state: file
    content: |
      {
        "env": "{{ .env.ENVIRONMENT }}",
        "host": "{{ .vars.hostname }}",
        "timestamp": "{{ now }}"
      }
    depends_on: [setup]

  - id: create_readme
    name: Generate README
    type: file
    path: "{{ .vars.temp_dir }}/README.md"
    state: file
    content: |
      # Project
      Created: {{ now }}
      Host: {{ .vars.hostname }}
    depends_on: [setup]

  # Create archive after both files are ready
  - id: archive
    name: Create Archive
    type: compress
    path: "{{ .vars.temp_dir }}.tar.gz"
    state: create
    format: tar.gz
    sources:
      - "{{ .vars.temp_dir }}"
    depends_on: [create_config, create_readme]

  # Calculate checksum
  - id: checksum
    name: Calculate Archive Checksum
    type: checksum
    path: "{{ .vars.temp_dir }}.tar.gz"
    algorithm: sha256
    depends_on: [archive]
    register: archive_checksum

  # Cleanup
  - id: cleanup
    name: Cleanup
    type: file
    path: "{{ .vars.temp_dir }}"
    state: absent
    depends_on: [checksum]
```

## 🔧 Task Configuration

### Command Task

```yaml
- name: Run Script
  type: command
  command: "./deploy.sh"
  working_dir: "/opt/app"
  environment:
    ENV: "production"
  timeout: "5m"
```

### File Task

```yaml
- name: Create File
  type: file
  path: "/etc/config/app.conf"
  state: file
  content: |
    server_name={{ .vars.hostname }}
    port=8080
  mode: "0644"
  owner: "app"
  group: "app"
```

### SSH Task

```yaml
- name: Deploy to Remote
  type: ssh
  host: "production.example.com"
  user: "deploy"
  key_file: "~/.ssh/id_rsa"
  command: "cd /app && git pull && systemctl restart app"
```

### Email Task

```yaml
- name: Send Notification
  type: email
  host: "smtp.gmail.com"
  port: 587
  username: "alerts@example.com"
  password: "${SMTP_PASSWORD}"
  from: "alerts@example.com"
  to: ["team@example.com"]
  subject: "Deployment Complete"
  body: "Deployment finished at {{ now }}"
  use_tls: true
```

### Slack Task

```yaml
- name: Notify Team
  type: slack
  webhook_url: "${SLACK_WEBHOOK}"
  message: "Deployment to production completed successfully!"
  channel: "#deployments"
  username: "Ritual Bot"
  icon_emoji: ":rocket:"
  color: "good"
```

### Checksum Task

```yaml
- name: Verify File
  type: checksum
  path: "/downloads/package.tar.gz"
  algorithm: sha256
  action: verify
  expected: "abc123..."
```

### Compress Task

```yaml
- name: Create Backup
  type: compress
  path: "/backups/data-{{ .vars.timestamp }}.tar.gz"
  state: create
  format: tar.gz
  sources:
    - "/var/lib/data"
    - "/etc/config"
```

## 🎯 Features in Detail

### Parallel Execution

Tasks without dependencies run in parallel automatically:

```yaml
tasks:
  - id: task1
    name: Independent Task 1
    type: command
    command: sleep 1

  - id: task2
    name: Independent Task 2
    type: command
    command: sleep 1

  # These two tasks run simultaneously!
```

### Dependency Management

Use `depends_on` to control execution order:

```yaml
tasks:
  - id: build
    name: Build App
    type: command
    command: make build

  - id: test
    name: Run Tests
    type: command
    command: make test
    depends_on: [build]

  - id: deploy
    name: Deploy
    type: command
    command: make deploy
    depends_on: [build, test]
```

### Template Variables

Access variables and context in templates:

```yaml
vars:
  app_name: "MyApp"
  version: "1.0.0"

tasks:
  - name: Print Info
    type: command
    command: |
      echo "App: {{ .vars.app_name }}"
      echo "Version: {{ .vars.version }}"
      echo "Host: {{ hostname }}"
      echo "Time: {{ now }}"
```

### Conditional Execution

Skip tasks based on conditions:

```yaml
- name: Optional Task
  type: command
  command: echo "Running in production"
  when: "{{ eq .env.ENVIRONMENT \"production\" }}"
```

### Task Results

Register and use task outputs:

```yaml
- id: get_version
  name: Get Version
  type: command
  command: cat VERSION
  register: version_result

- id: use_version
  name: Use Version
  type: command
  command: echo "Version is {{ .tasks.get_version.Stdout }}"
  depends_on: [get_version]
```

### Required vs Optional Tasks

Control workflow failure behavior:

```yaml
- name: Critical Task
  type: command
  command: ./deploy.sh
  required: true  # Workflow fails if this fails

- name: Optional Notification
  type: slack
  webhook_url: "${SLACK_WEBHOOK}"
  message: "Deployment complete"
  required: false  # Workflow continues if this fails
```

## 🔍 CLI Reference

### Global Flags

```bash
--config string   # Config file (default: $HOME/.ritual.yaml)
--format string   # Output format: text, json (default: "text")
-v, --verbose     # Enable verbose output
-q, --quiet       # Quiet mode (errors only)
--version         # Show version
```

### Commands

#### run
Execute a workflow

```bash
ritual run workflow.yaml [flags]

Flags:
  --mode string           # Execution mode: parallel, sequential (default: "parallel")
  --var stringArray       # Set variables: --var key=value
  --env-file string       # Load environment from file
```

#### validate
Validate workflow syntax and dependencies

```bash
ritual validate workflow.yaml
```

#### dry-run
Preview execution without making changes

```bash
ritual dry-run workflow.yaml [flags]

Flags:
  --format string   # Output format: text, json (default: "text")
  --var stringArray # Set variables
```

#### list-tasks
Show available task types

```bash
ritual list-tasks
```

## 📚 Examples

See the `examples/` directory for complete workflow examples:

- `simple.yaml` - Basic workflow with two tasks
- `file-ops.yaml` - File operations and archiving
- `showcase.yaml` - Comprehensive feature demonstration

## 🏗️ Architecture

```
cmd/
  ritual/          # Main CLI application
internal/
  cli/             # Cobra command implementations
  orchestrator/    # Workflow coordination
  executor/        # Task execution engine
  tasks/           # Task type implementations
  workflow/        # Parser and resolver
  template/        # Template engine
  context/         # Context management
pkg/
  types/           # Core types and interfaces
```

## 🧪 Development

### Building

```bash
go build -o ritual ./cmd/ritual
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/tasks/...

# Run with coverage
go test -cover ./...
```

### Adding Custom Task Types

1. Create a new package in `internal/tasks/`
2. Implement the `types.TaskExecutor` interface
3. Register in `internal/tasks/registry.go`

Example:

```go
package mytask

type Executor struct{}

func New() *Executor {
    return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig,
    contextManager types.ContextManager) *types.TaskResult {
    // Implementation
}

func (e *Executor) Validate(task *types.TaskConfig) error {
    // Validation
}

func (e *Executor) SupportsDryRun() bool {
    return true
}
```

## 🤝 Contributing

Contributions welcome! Please ensure:

- All tests pass
- Code follows Go conventions
- New features include tests
- Documentation is updated

## 📄 License

[Add your license here]

## 🙏 Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration
- [Sprig](https://github.com/Masterminds/sprig) - Template functions
- [Zerolog](https://github.com/rs/zerolog) - Structured logging
- [Afero](https://github.com/spf13/afero) - Filesystem abstraction

---

Made with ❤️ by the Ritual team
