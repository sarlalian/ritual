# 🔮 Ritual - Declarative Workflow Automation Engine

A powerful CLI-based workflow engine written in Go that executes declarative YAML workflows ("incantations") with built-in parallel execution, dependency resolution, cross-filesystem operations, and rich templating capabilities.

## ✨ Features

- **⚡ Parallel Execution** - Execute independent tasks concurrently with configurable concurrency limits
- **🔗 Smart Dependency Resolution** - Automatic topological sorting and execution layering
- **📝 Rich Templating** - Built-in Sprig functions with access to variables, environment, and task results
- **🎯 22 Task Types** - 10 core task types with 12 aliases covering most automation needs
- **🌍 Cross-Filesystem Operations** - Copy files between local, S3, and SFTP seamlessly
- **🔄 Context Sharing** - Share variables and task results between steps
- **🧪 Dry-Run Mode** - Preview workflows without making changes
- **📊 Structured Logging** - Comprehensive logging with multiple verbosity levels
- **🎨 Import System** - Compose workflows from multiple sources (local, HTTP, S3, Git, SSH)
- **✅ Comprehensive Testing** - 18 test suites covering all components
- **📚 Extensive Examples** - 19 example workflows demonstrating all features

## 🚀 Quick Start

### Installation

```bash
# Build from source
go build -o ritual ./cmd/ritual

# Or use go install
go install github.com/sarlalian/ritual/cmd/ritual@latest
```

### Basic Usage

```bash
# Execute a workflow
ritual run workflow.yaml

# Validate workflow syntax
ritual validate workflow.yaml

# Preview execution without running
ritual dry-run workflow.yaml

# List available task types
ritual list-tasks

# Get help
ritual --help
```

## 📋 Task Types

### Core Tasks
- **command** / **shell** / **script** - Execute shell commands and scripts with timeout support
- **file** / **template** - File operations (create, copy, delete, chmod, chown, templating)
- **copy** - Cross-filesystem file copying (local, S3, SFTP/SSH)

### Compression
- **compress** / **archive** / **unarchive** - Create and extract archives (tar, gzip, zip, bzip2, LZMA2)

### Security
- **checksum** / **hash** / **verify** - Calculate and verify file checksums (SHA256, SHA512, MD5, Blake2b)

### Remote Operations
- **ssh** / **remote** - Execute commands on remote hosts via SSH with key or password auth

### Communication
- **email** / **mail** - Send emails via SMTP with TLS support and attachments
- **ses** / **aws_email** - Send emails via Amazon SES with template support
- **slack** / **notify** - Post rich messages to Slack channels via webhooks

### Debugging
- **debug** / **log** - Log templated messages for workflow debugging

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

### Cross-Filesystem Copy Example

```yaml
name: Cross-Filesystem Copy
description: Copy files between local, S3, and SFTP

vars:
  local_dir: /tmp/data
  s3_bucket: my-backup-bucket
  s3_region: us-east-1

tasks:
  # Copy local file to S3
  - id: backup_to_s3
    name: Backup to S3
    type: copy
    src: "{{ .vars.local_dir }}/database.sql"
    dest: "s3://{{ .vars.s3_bucket }}/backups/database.sql"
    aws_region: "{{ .vars.s3_region }}"

  # Copy from SFTP to local
  - id: fetch_from_sftp
    name: Fetch from SFTP
    type: copy
    src: "sftp://backup-server.com/archive/logs.tar.gz"
    dest: "{{ .vars.local_dir }}/logs.tar.gz"
    ssh_user: backup
    ssh_private_key_path: ~/.ssh/id_rsa
```

### Advanced Workflow Example

```yaml
name: Advanced Build & Deploy Workflow
description: Demonstrates parallel execution, dependencies, and templating
version: "1.0"

environment:
  ENVIRONMENT: "production"
  API_KEY: "${API_KEY}"

vars:
  temp_dir: "/tmp/ritual-build"
  hostname: "{{ hostname }}"
  build_time: "{{ now | date \"2006-01-02-15:04:05\" }}"

tasks:
  # Setup
  - id: setup
    name: Setup Build Directory
    type: file
    path: "{{ .vars.temp_dir }}"
    state: directory
    mode: "0755"

  # Parallel build steps (both depend on setup)
  - id: build_frontend
    name: Build Frontend
    type: command
    command: npm run build
    working_dir: ./frontend
    depends_on: [setup]

  - id: build_backend
    name: Build Backend
    type: command
    command: go build -o app ./cmd/server
    working_dir: ./backend
    depends_on: [setup]

  # Generate config (parallel with builds)
  - id: generate_config
    name: Generate Configuration
    type: file
    path: "{{ .vars.temp_dir }}/config.json"
    state: file
    content: |
      {
        "environment": "{{ .env.ENVIRONMENT }}",
        "host": "{{ .vars.hostname }}",
        "build_time": "{{ .vars.build_time }}",
        "version": "1.0.0"
      }
    depends_on: [setup]

  # Create deployment package (after builds complete)
  - id: create_package
    name: Create Deployment Package
    type: compress
    path: "{{ .vars.temp_dir }}/deploy-{{ .vars.build_time }}.tar.gz"
    state: create
    format: tar.gz
    sources:
      - ./frontend/dist
      - ./backend/app
      - "{{ .vars.temp_dir }}/config.json"
    depends_on: [build_frontend, build_backend, generate_config]

  # Calculate checksum
  - id: checksum
    name: Calculate Package Checksum
    type: checksum
    path: "{{ .vars.temp_dir }}/deploy-{{ .vars.build_time }}.tar.gz"
    algorithm: sha256
    action: calculate
    depends_on: [create_package]
    register: package_checksum

  # Upload to S3
  - id: upload_package
    name: Upload to S3
    type: copy
    src: "{{ .vars.temp_dir }}/deploy-{{ .vars.build_time }}.tar.gz"
    dest: "s3://my-deploys/packages/deploy-{{ .vars.build_time }}.tar.gz"
    aws_region: us-east-1
    depends_on: [checksum]

  # Deploy via SSH
  - id: deploy
    name: Deploy to Production
    type: ssh
    host: production-server.example.com
    user: deploy
    key_file: ~/.ssh/deploy_key
    command: |
      cd /opt/app
      aws s3 cp s3://my-deploys/packages/deploy-{{ .vars.build_time }}.tar.gz .
      tar xzf deploy-{{ .vars.build_time }}.tar.gz
      sudo systemctl restart app
    depends_on: [upload_package]

  # Send notifications (parallel)
  - id: notify_slack
    name: Notify Slack
    type: slack
    webhook_url: "${SLACK_WEBHOOK}"
    message: |
      🚀 Deployment Complete!
      Environment: {{ .env.ENVIRONMENT }}
      Build Time: {{ .vars.build_time }}
      Checksum: {{ .tasks.checksum.Output.checksum }}
    color: "good"
    depends_on: [deploy]

  - id: notify_email
    name: Send Email Notification
    type: ses
    region: us-east-1
    from: "deploys@example.com"
    to: ["team@example.com"]
    subject: "Production Deployment Complete - {{ .vars.build_time }}"
    body: |
      Deployment to production completed successfully.

      Build Time: {{ .vars.build_time }}
      Host: {{ .vars.hostname }}
      Package Checksum: {{ .tasks.checksum.Output.checksum }}

      All systems operational.
    depends_on: [deploy]

  # Cleanup
  - id: cleanup
    name: Cleanup Build Directory
    type: file
    path: "{{ .vars.temp_dir }}"
    state: absent
    depends_on: [notify_slack, notify_email]
    required: false  # Don't fail workflow if cleanup fails
```

## 🔧 Task Configuration Reference

### Command Task

Execute shell commands with full environment control:

```yaml
- name: Run Deployment Script
  type: command
  command: "./deploy.sh --env production"
  working_dir: "/opt/app"
  environment:
    ENV: "production"
    DATABASE_URL: "{{ .vars.db_url }}"
  timeout: "10m"
  shell: "/bin/bash"
```

### File Task

Comprehensive file operations:

```yaml
- name: Create Configuration File
  type: file
  path: "/etc/app/config.yaml"
  state: file  # Options: file, directory, absent, link
  content: |
    server:
      host: {{ .vars.hostname }}
      port: 8080
    database:
      url: {{ .env.DATABASE_URL }}
  mode: "0644"
  owner: "app"
  group: "app"
  backup: true
```

### Copy Task (Cross-Filesystem)

Copy files between local, S3, and SFTP:

```yaml
# Local to S3
- name: Upload to S3
  type: copy
  src: "/local/path/file.txt"
  dest: "s3://bucket-name/path/file.txt"
  aws_region: "us-east-1"
  aws_access_key_id: "{{ .env.AWS_ACCESS_KEY_ID }}"
  aws_secret_access_key: "{{ .env.AWS_SECRET_ACCESS_KEY }}"
  force: true  # Overwrite if exists

# SFTP to Local
- name: Download from SFTP
  type: copy
  src: "sftp://remote-host.com/data/backup.tar.gz"
  dest: "/local/backups/backup.tar.gz"
  ssh_user: "backup-user"
  ssh_private_key_path: "~/.ssh/id_rsa"
  # OR: ssh_password: "{{ .env.SSH_PASSWORD }}"
  recursive: true  # For directories
  create_dirs: true  # Create destination directories
```

### SSH Task

Execute commands on remote hosts:

```yaml
- name: Deploy to Remote Server
  type: ssh
  host: "production.example.com"
  port: 22
  user: "deploy"
  key_file: "~/.ssh/deploy_key"
  # OR: password: "{{ .env.SSH_PASSWORD }}"
  command: |
    cd /opt/app
    git pull origin main
    npm install --production
    pm2 restart app
  timeout: "5m"
```

### Email Task (SMTP)

Send emails via SMTP with TLS:

```yaml
- name: Send Alert Email
  type: email
  host: "smtp.gmail.com"
  port: 587
  username: "alerts@example.com"
  password: "{{ .env.SMTP_PASSWORD }}"
  from: "alerts@example.com"
  to: ["admin@example.com", "ops@example.com"]
  cc: ["team@example.com"]
  subject: "Alert: {{ .vars.alert_type }}"
  body: |
    Alert occurred at {{ now }}

    Details:
    {{ .vars.alert_details }}
  use_tls: true
  attachments:
    - "/logs/error.log"
    - "/reports/summary.pdf"
```

### SES Task (Amazon SES)

Send emails via Amazon SES:

```yaml
- name: Send SES Email
  type: ses
  region: "us-east-1"
  from: "noreply@example.com"
  to: ["customer@example.com"]
  subject: "Welcome to Our Service"
  body_html: |
    <html>
      <body>
        <h1>Welcome!</h1>
        <p>Thanks for signing up.</p>
      </body>
    </html>
  body: "Welcome! Thanks for signing up."
  configuration_set: "my-config-set"
  tags:
    Campaign: "welcome-series"
    Environment: "production"
```

### Slack Task

Post rich messages to Slack:

```yaml
- name: Notify Team on Slack
  type: slack
  webhook_url: "{{ .env.SLACK_WEBHOOK }}"
  message: "Deployment completed successfully! 🎉"
  channel: "#deployments"
  username: "Ritual Bot"
  icon_emoji: ":rocket:"
  color: "good"
  fields:
    - title: "Environment"
      value: "{{ .env.ENVIRONMENT }}"
      short: true
    - title: "Version"
      value: "{{ .vars.version }}"
      short: true
```

### Checksum Task

Calculate and verify file checksums:

```yaml
# Calculate checksum
- name: Calculate Checksum
  type: checksum
  path: "/downloads/package.tar.gz"
  algorithm: sha256  # Options: sha256, sha512, md5, blake2b
  action: calculate
  register: package_hash

# Verify checksum
- name: Verify Package
  type: checksum
  path: "/downloads/package.tar.gz"
  algorithm: sha256
  action: verify
  expected: "abc123..."
```

### Compress Task

Create and extract archives:

```yaml
# Create archive
- name: Create Backup Archive
  type: compress
  path: "/backups/data-{{ .vars.timestamp }}.tar.gz"
  state: create
  format: tar.gz  # Options: tar, tar.gz, tar.bz2, zip, tar.xz
  sources:
    - "/var/lib/database"
    - "/etc/config"
    - "/opt/app"

# Extract archive
- name: Extract Archive
  type: compress
  path: "/downloads/backup.tar.gz"
  state: extract
  destination: "/restore/data"
```

### Debug Task

Log messages for debugging:

```yaml
- name: Debug Variables
  type: debug
  message: |
    Current Variables:
    - Environment: {{ .env.ENVIRONMENT }}
    - Hostname: {{ hostname }}
    - Build Time: {{ now }}
    - Previous Task Status: {{ .tasks.build.Status }}
  level: info  # Options: debug, info, warn, error
```

## 🎯 Advanced Features

### Parallel Execution with Concurrency Control

```bash
# Run with custom concurrency limit
ritual run workflow.yaml --max-concurrency 5

# Sequential execution
ritual run workflow.yaml --mode sequential
```

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

  # These run simultaneously (up to concurrency limit)
```

### Dependency Management

Use `depends_on` to control execution order:

```yaml
tasks:
  - id: build
    name: Build Application
    type: command
    command: make build

  - id: test
    name: Run Tests
    type: command
    command: make test
    depends_on: [build]

  - id: deploy
    name: Deploy to Production
    type: command
    command: make deploy
    depends_on: [build, test]  # Multiple dependencies
```

### Template Variables and Functions

Access rich context in templates:

```yaml
vars:
  app_name: "MyApp"
  version: "1.0.0"
  replicas: 3

tasks:
  - name: Print Info
    type: command
    command: |
      echo "App: {{ .vars.app_name }}"
      echo "Version: {{ .vars.version }}"
      echo "Host: {{ hostname }}"
      echo "Time: {{ now | date \"2006-01-02 15:04:05\" }}"
      echo "Uppercase: {{ .vars.app_name | upper }}"
      echo "Replicas: {{ .vars.replicas }}"
```

Supported template functions (via Sprig):
- Date/Time: `now`, `date`, `dateModify`, `ago`
- Strings: `upper`, `lower`, `title`, `trim`, `replace`
- Math: `add`, `sub`, `mul`, `div`, `mod`
- Collections: `list`, `dict`, `merge`, `keys`, `values`
- Conditionals: `if`, `eq`, `ne`, `lt`, `gt`, `and`, `or`
- And 100+ more from [Sprig](http://masterminds.github.io/sprig/)

### Conditional Execution

Skip tasks based on conditions:

```yaml
- name: Production Only Task
  type: command
  command: echo "Running in production"
  when: "{{ eq .env.ENVIRONMENT \"production\" }}"

- name: Non-Empty Variable Check
  type: command
  command: echo "API key is set"
  when: "{{ .env.API_KEY }}"
```

### Task Results and Context

Register and use task outputs:

```yaml
- id: get_version
  name: Get Application Version
  type: command
  command: cat VERSION
  register: app_version

- id: build
  name: Build with Version
  type: command
  command: |
    echo "Building version: {{ .tasks.get_version.Stdout }}"
    make build VERSION={{ .tasks.get_version.Stdout | trim }}
  depends_on: [get_version]

- id: report
  name: Build Report
  type: debug
  message: |
    Build Status: {{ .tasks.build.Status }}
    Exit Code: {{ .tasks.build.ExitCode }}
    Duration: {{ .tasks.build.Duration }}
  depends_on: [build]
```

### Required vs Optional Tasks

Control workflow failure behavior:

```yaml
- name: Critical Deployment
  type: command
  command: ./deploy.sh
  required: true  # Workflow fails if this fails (default)

- name: Send Notification
  type: slack
  webhook_url: "${SLACK_WEBHOOK}"
  message: "Deployment complete"
  required: false  # Workflow continues even if this fails
```

### Workflow Imports

Compose workflows from multiple sources:

```yaml
name: Composed Workflow
description: Imports tasks from multiple sources

imports:
  # Import from local file
  - path: ./library/common-tasks.yaml

  # Import from HTTP
  - path: https://example.com/workflows/deploy.yaml

  # Import from S3
  - path: s3://my-bucket/workflows/tests.yaml
    aws_region: us-east-1

  # Import from Git
  - path: git://github.com/user/repo/workflows/ci.yaml

  # Import from SSH
  - path: ssh://host.com/path/to/workflow.yaml
    ssh_user: deploy

tasks:
  - id: use_imported
    name: Use Imported Task
    type: command
    command: echo "Using tasks from imports"
    depends_on: [imported_task_id]
```

### Variable Files

Load variables from external files:

```bash
# Load variables from file
ritual run workflow.yaml --var-file production.yaml

# Override with CLI variables
ritual run workflow.yaml --var-file prod.yaml --var key=value
```

```yaml
# production.yaml
database_url: postgres://prod-db:5432/app
api_endpoint: https://api.production.com
replicas: 5
```

### Environment Variables

Load environment from files:

```bash
ritual run workflow.yaml --env-file .env.production
```

```
# .env.production
DATABASE_URL=postgres://prod:5432/db
API_KEY=secret-key-here
SLACK_WEBHOOK=https://hooks.slack.com/...
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

Execute a workflow with full control:

```bash
ritual run workflow.yaml [flags]

Flags:
  --max-concurrency int     # Max concurrent tasks (default: 10)
  --mode string             # Execution mode: parallel, sequential (default: "parallel")
  --var stringArray         # Set variables: --var key=value
  --var-file string         # Load variables from YAML file
  --env-file string         # Load environment from file
  --dry-run                 # Preview without execution
```

Examples:
```bash
# Basic execution
ritual run deploy.yaml

# With custom concurrency
ritual run deploy.yaml --max-concurrency 20

# Sequential execution
ritual run deploy.yaml --mode sequential

# With variables
ritual run deploy.yaml --var environment=prod --var version=1.2.3

# With variable file
ritual run deploy.yaml --var-file production.yaml

# Dry run
ritual run deploy.yaml --dry-run
```

#### validate

Validate workflow syntax and dependencies:

```bash
ritual validate workflow.yaml

# Returns exit code 0 if valid, non-zero if invalid
# Shows detailed error messages for issues
```

#### dry-run

Preview execution plan without making changes:

```bash
ritual dry-run workflow.yaml [flags]

Flags:
  --format string   # Output format: text, json (default: "text")
  --var stringArray # Set variables
  --var-file string # Load variables from file
```

#### list-tasks

Show all available task types:

```bash
ritual list-tasks

# Output shows:
# - Task type names
# - Aliases
# - Brief descriptions
# - Organized by category
```

## 📚 Examples

The `examples/` directory contains 19+ comprehensive workflow examples:

**Core Examples:**
- `simple.yaml` - Basic two-task workflow
- `showcase.yaml` - Comprehensive feature demonstration
- `conditional-deployment.yaml` - Conditional execution patterns

**File Operations:**
- `file-ops.yaml` - File creation, templating, permissions
- `copy-example.yaml` - Local file copying with backup
- `copy-cross-filesystem.yaml` - S3 and SFTP operations

**System Administration:**
- `backup-system.yaml` - Complete backup workflow
- `mysql-backup-system.yaml` - Database backup example

**Communication:**
- `ses-example.yaml` - Amazon SES email examples with templates

**Advanced:**
- `simple-with-imports.yaml` - Workflow composition
- `composed-simple.yaml` - Multiple imports
- `variable-test.yaml` - Variable substitution patterns
- `variable-file-demo.yaml` - External variable files

**Testing:**
- `test-debug-task.yaml` - Debug task usage
- `test-command-logging.yaml` - Command output handling
- `test-failure.yaml` - Error handling

**Libraries:**
- `library/common-tasks.yaml` - Reusable task definitions
- `library/nodejs-tasks.yaml` - Node.js specific tasks

## 🏗️ Architecture

```
cmd/
  ritual/              # Main CLI application entry point
internal/
  cli/                 # Cobra command implementations
    run.go             # Run command
    validate.go        # Validation command
    dry_run.go         # Dry-run command
    list_tasks.go      # Task listing
  orchestrator/        # Workflow coordination and execution
  executor/            # Task execution engine with concurrency
  tasks/               # Task type implementations
    command/           # Shell command execution
    file/              # File operations
    copy/              # Cross-filesystem copying
    compress/          # Archive operations
    checksum/          # Hash calculations
    ssh/               # Remote execution
    email/             # SMTP email
    ses/               # Amazon SES
    slack/             # Slack notifications
    debug/             # Debug logging
    registry.go        # Task type registry
  workflow/            # Workflow parsing and resolution
    parser/            # YAML parser
    resolver/          # Dependency resolver
    imports/           # Import system
  template/            # Template engine with Sprig
  context/             # Context and variable management
  filesystem/          # Filesystem abstraction (S3, SFTP, local)
pkg/
  types/               # Core types and interfaces
  utils/               # Utility functions
test/
  integration/         # Integration tests
examples/              # 19+ example workflows
docs/                  # Additional documentation
```

## 🧪 Development

### Building

```bash
# Build binary
go build -o ritual ./cmd/ritual

# Build with version info
go build -ldflags "-X main.version=1.0.0" -o ritual ./cmd/ritual

# Install to $GOPATH/bin
go install ./cmd/ritual
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/tasks/copy/...

# Run integration tests
go test ./test/integration/...

# Verbose test output
go test -v ./internal/executor/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Coverage

The project includes comprehensive test coverage:

- **18 Test Suites** covering all major components
- **Unit Tests** for each task type
- **Integration Tests** for end-to-end workflows
- **Parser Tests** for YAML validation
- **Template Tests** for rendering
- **Context Tests** for variable management

### Adding Custom Task Types

1. Create a new package in `internal/tasks/`:

```go
package mytask

import (
    "context"
    "github.com/sarlalian/ritual/pkg/types"
)

type Executor struct{}

func New() *Executor {
    return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, task *types.TaskConfig,
    contextManager types.ContextManager) *types.TaskResult {

    result := &types.TaskResult{
        ID:        task.ID,
        Name:      task.Name,
        Type:      task.Type,
        StartTime: time.Now(),
        Status:    types.TaskRunning,
        Output:    make(map[string]interface{}),
    }

    // Your implementation here

    result.Status = types.TaskSuccess
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    return result
}

func (e *Executor) Validate(task *types.TaskConfig) error {
    // Validation logic
    return nil
}

func (e *Executor) SupportsDryRun() bool {
    return true
}
```

2. Register in `internal/tasks/registry.go`:

```go
import "github.com/sarlalian/ritual/internal/tasks/mytask"

func (r *Registry) RegisterBuiltinTasks() {
    // ... existing registrations
    r.Register("mytask", mytask.New())
}
```

3. Add to `internal/cli/list_tasks.go`:

```go
descriptions := map[string]string{
    // ... existing descriptions
    "mytask": "Description of my custom task",
}

categories := map[string][]string{
    // ... existing categories
    "Custom": {"mytask"},
}
```

4. Write tests in `internal/tasks/mytask/mytask_test.go`

## 🤝 Contributing

Contributions welcome! Please ensure:

1. **All tests pass**: `go test ./...`
2. **Code follows Go conventions**: `go fmt ./...` and `go vet ./...`
3. **New features include tests**: Maintain test coverage
4. **Documentation is updated**: Update README and add examples
5. **Commit messages are clear**: Use conventional commit format

### Development Workflow

```bash
# 1. Fork and clone the repository
git clone https://github.com/yourusername/ritual.git
cd ritual

# 2. Create a feature branch
git checkout -b feature/my-new-feature

# 3. Make your changes and test
go test ./...

# 4. Format and vet your code
go fmt ./...
go vet ./...

# 5. Commit your changes
git add .
git commit -m "feat: add my new feature"

# 6. Push and create pull request
git push origin feature/my-new-feature
```

## 📄 License

[Add your license here]

## 🙏 Acknowledgments

Built with excellent open-source libraries:

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [Sprig](https://github.com/Masterminds/sprig) - Template functions
- [Zerolog](https://github.com/rs/zerolog) - Structured logging
- [Afero](https://github.com/spf13/afero) - Filesystem abstraction
- [afero-s3](https://github.com/fclairamb/afero-s3) - S3 filesystem support
- [pkg/sftp](https://github.com/pkg/sftp) - SFTP client
- [AWS SDK for Go](https://github.com/aws/aws-sdk-go) - AWS service integration

## 📖 Additional Documentation

See the `docs/` directory for:
- `FEATURES.md` - Detailed feature documentation
- `PROJECT_STATUS.md` - Current project status
- `COMPLETION_SUMMARY.md` - Implementation progress

---

**Made with ❤️ using Claude Code**
