# Ritual System Design Reference

This document provides a comprehensive architectural overview of the Ritual workflow engine, including ASCII diagrams showing how the major components interact.

## Table of Contents

1. [High-Level Architecture](#high-level-architecture)
2. [Component Interactions](#component-interactions)
3. [Core Systems](#core-systems)
   - [CLI System](#cli-system)
   - [Orchestrator](#orchestrator)
   - [Executor](#executor)
   - [Dependency Resolver](#dependency-resolver)
   - [Context Manager](#context-manager)
   - [Task Registry](#task-registry)
4. [Supporting Systems](#supporting-systems)
   - [Filesystem Factory](#filesystem-factory)
   - [Template Engine](#template-engine)
   - [Import Resolver](#import-resolver)
   - [Event Systems](#event-systems)
5. [Execution Flow](#execution-flow)
6. [Data Flow](#data-flow)

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            RITUAL WORKFLOW ENGINE                       │
└─────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│                           ENTRY POINTS                                   │
├───────────────┬───────────────┬──────────────┬──────────────┬────────────┤
│  CLI Commands │ HTTP Webhooks │  File Watch  │  Pub/Sub     │  Direct API│
│   (Cobra)     │   (Server)    │  (Watcher)   │  (Events)    │   (Go)     │
└───────┬───────┴───────┬───────┴──────┬───────┴──────┬───────┴────┬───────┘
        │               │              │              │            │
        └───────────────┴──────────────┴──────────────┴────────────┘
                                    │
                                    ▼
                    ┌───────────────────────────────┐
                    │      ORCHESTRATOR             │
                    │  (Workflow Coordinator)       │
                    └───────────────┬───────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐          ┌────────────────┐         ┌─────────────────┐
│    PARSER     │          │    RESOLVER    │         │     EXECUTOR    │
│ (YAML → AST)  │          │  (Dependency   │         │  (Task Runner)  │
│               │          │   Graph + DAG) │         │                 │
└───────┬───────┘          └────────┬───────┘         └────────┬────────┘
        │                           │                          │
        │                           │                          │
        ▼                           ▼                          ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        SUPPORTING SYSTEMS                              │
├──────────────┬─────────────────┬──────────────────┬────────────────────┤
│   Context    │    Template     │   Task Registry  │   Filesystem       │
│   Manager    │    Engine       │   (Built-in +    │   Factory          │
│  (Variables, │   (Sprig +      │    Custom)       │   (Local, S3,      │
│   Results)   │   Go Templates) │                  │    SSH, HTTP)      │
└──────────────┴─────────────────┴──────────────────┴────────────────────┘
```

---

## Component Interactions

### Complete Workflow Execution Flow

```
  User Input (YAML)
       │
       ▼
┌─────────────┐
│     CLI     │ ritual run workflow.yaml
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────┐
│                    ORCHESTRATOR                             │
│  1. Initialize all components                               │
│  2. Coordinate workflow execution                           │
└────┬──────┬──────┬───────┬───────┬───────┬──────────────────┘
     │      │      │       │       │       │
     │      │      │       │       │       └──────┐
     ▼      ▼      ▼       ▼       ▼              ▼
  Parser  Resolver Context Template Task      History
           (DAG)   Manager Engine  Registry   Store
     │      │       │       │       │            │
     └──────┴───────┴───────┴───────┴────────────┘
                        │
                        ▼
              ┌─────────────────┐
              │    EXECUTOR     │
              │  Execute Layers │
              └────────┬────────┘
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
   Layer 0        Layer 1        Layer 2
 ┌─────────┐    ┌─────────┐    ┌─────────┐
 │ Task A  │    │ Task B  │    │ Task D  │
 │ Task C  │    │         │    │         │
 └─────────┘    └─────────┘    └─────────┘
   Parallel      Parallel       Parallel
  (No deps)   (Depends on A)  (Depends on B)
        │              │              │
        └──────────────┴──────────────┘
                       │
                       ▼
               ┌───────────────┐
               │  Task Results │
               │  + Outputs    │
               └───────────────┘
```

---

## Core Systems

### CLI System

The CLI system provides the command-line interface using Cobra framework.

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI ARCHITECTURE                          │
└─────────────────────────────────────────────────────────────┘

          ┌──────────────┐
          │   rootCmd    │  (Cobra Command)
          │  "ritual"    │
          └──────┬───────┘
                 │
    ┌────────────┼────────────┬────────────┬──────────────┐
    │            │            │            │              │
    ▼            ▼            ▼            ▼              ▼
┌────────┐  ┌────────┐  ┌─────────┐  ┌────────┐    ┌─────────┐
│  run   │  │dry-run │  │validate │  │ serve  │    │  watch  │
└───┬────┘  └───┬────┘  └────┬────┘  └───┬────┘    └────┬────┘
    │           │            │           │              │
    └───────────┴────────────┴───────────┴──────────────┘
                          │
                          ▼
              ┌──────────────────────┐
              │   Config & Logger    │
              │   Initialization     │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │   Orchestrator       │
              │   Creation           │
              └──────────────────────┘

Flags:
  --config   : Config file path
  --verbose  : Enable debug logging
  --quiet    : Suppress non-error output
  --format   : Output format (text/json)
  --dry-run  : Simulation mode
```

### Orchestrator

The Orchestrator is the central coordinator that integrates all components.

```
┌──────────────────────────────────────────────────────────────────┐
│                    ORCHESTRATOR INTERNALS                         │
└──────────────────────────────────────────────────────────────────┘

                    ┌───────────────────┐
                    │   Orchestrator    │
                    │    (Facade)       │
                    └─────────┬─────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
      ┌──────────────┐ ┌─────────────┐ ┌────────────┐
      │   Parser     │ │  Resolver   │ │  Executor  │
      │  (Workflow   │ │ (Dependency │ │   (Task    │
      │   → AST)     │ │   Graph)    │ │  Runner)   │
      └──────┬───────┘ └──────┬──────┘ └─────┬──────┘
             │                │              │
             │                │              │
      ┌──────┴────────────────┴──────────────┴──────┐
      │                                              │
      ▼                                              ▼
┌──────────────┐                            ┌───────────────┐
│   Context    │◄───────────────────────────│  Task Results │
│   Manager    │                            │   Collection  │
└──────────────┘                            └───────────────┘
      │                                              │
      ▼                                              ▼
┌──────────────────────────────────────────────────────────┐
│                  Template Engine                         │
│  Variables, Results, Workflow Data → Rendered Templates  │
└──────────────────────────────────────────────────────────┘

Orchestrator Responsibilities:
  • Component initialization & lifecycle
  • Workflow import resolution
  • Error aggregation and reporting
  • History/audit logging
  • Dry-run coordination
```

### Executor

The Executor manages task execution with parallel and sequential modes.

```
┌──────────────────────────────────────────────────────────────┐
│                  EXECUTOR ARCHITECTURE                        │
└──────────────────────────────────────────────────────────────┘

                    ┌──────────────┐
                    │   Executor   │
                    └──────┬───────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌──────────────┐   ┌──────────────┐   ┌─────────────┐
│ Task Registry│   │   Context    │   │   Config    │
│  (Type Map)  │   │   Manager    │   │  (DryRun,   │
│              │   │  (Variables) │   │  MaxConc)   │
└──────┬───────┘   └──────┬───────┘   └──────┬──────┘
       │                  │                  │
       └──────────────────┴──────────────────┘
                          │
                          ▼
              ┌───────────────────────┐
              │  ExecuteWorkflow()    │
              └───────────┬───────────┘
                          │
                ┌─────────┴─────────┐
                ▼                   ▼
        Get Execution Layers    Validate Tasks
                │
                ▼
    ┌───────────────────────┐
    │  Layer-by-Layer Exec  │
    └───────────┬───────────┘
                │
    ┌───────────┴──────────┐
    ▼                      ▼
Sequential            Parallel
(Between Layers)    (Within Layer)
    │                      │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │  executeTasksLayer() │
    │                      │
    │  • Semaphore pool    │
    │  • Goroutines        │
    │  • WaitGroup         │
    │  • Result collect    │
    └──────────┬───────────┘
               │
               ▼
    ┌──────────────────────┐
    │   executeTask()      │
    │                      │
    │  1. Resolve task     │
    │  2. Get executor     │
    │  3. Run with context │
    │  4. Capture result   │
    │  5. Handle errors    │
    └──────────────────────┘

Concurrency Model:
  • MaxConcurrency = semaphore size (default 10)
  • Tasks in same layer run in parallel
  • Layers execute sequentially
  • Cancelled context propagates to all tasks
```

### Dependency Resolver

The Resolver builds dependency graphs and generates execution layers.

```
┌───────────────────────────────────────────────────────────────┐
│               DEPENDENCY RESOLVER ARCHITECTURE                 │
└───────────────────────────────────────────────────────────────┘

Input: []TaskConfig with DependsOn fields

        ┌─────────────────────┐
        │  BuildGraph()       │
        │                     │
        │  1. Create nodes    │
        │  2. Link deps       │
        │  3. Validate        │
        └──────────┬──────────┘
                   │
                   ▼
        ┌─────────────────────┐
        │   Dependency Graph   │
        │                     │
        │   Task A (InDeg=0)  │
        │      ↓              │
        │   Task B (InDeg=1)  │
        │      ↓              │
        │   Task C (InDeg=1)  │
        └──────────┬──────────┘
                   │
                   ▼
        ┌─────────────────────┐
        │  ResolveOrder()     │
        │  (Topological Sort) │
        │                     │
        │  • Kahn's Algorithm │
        │  • Queue processing │
        │  • Cycle detection  │
        └──────────┬──────────┘
                   │
                   ▼
        ┌─────────────────────┐
        │  Execution Layers   │
        ├─────────────────────┤
        │  Layer 0: [A, D]    │ ← No dependencies
        │  Layer 1: [B, E]    │ ← Depends on Layer 0
        │  Layer 2: [C]       │ ← Depends on Layer 1
        └─────────────────────┘

Graph Structure:
  • TaskNode: Task + metadata
  • Dependencies: []*TaskNode (tasks this depends on)
  • Dependents: []*TaskNode (tasks that depend on this)
  • InDegree: Count of dependencies
  • Layer: Execution layer number

Example Dependency Graph:
      A       D
      │       │
      ├───┬───┤
      │   │   │
      ▼   ▼   ▼
      B   E   F
      │   │   │
      └───┼───┘
          ▼
          C

Becomes:
  Layer 0: [A, D]      (Run in parallel)
  Layer 1: [B, E, F]   (Wait for A or D, then parallel)
  Layer 2: [C]         (Wait for B and E)
```

### Context Manager

Manages workflow variables, environment, and task results.

```
┌────────────────────────────────────────────────────────────┐
│              CONTEXT MANAGER ARCHITECTURE                   │
└────────────────────────────────────────────────────────────┘

                ┌─────────────────────┐
                │  Context Manager    │
                └──────────┬──────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
  ┌─────────────┐  ┌──────────────┐  ┌─────────────┐
  │  Variables  │  │  Task Results│  │   Workflow  │
  │   (Map)     │  │    (Map)     │  │   Metadata  │
  └─────────────┘  └──────────────┘  └─────────────┘

Context Data Structure:
┌────────────────────────────────────────────────────────────┐
│ .workflow         → Workflow metadata                      │
│   .name           → Workflow name                          │
│   .description    → Description                            │
│   .version        → Version                                │
│                                                            │
│ .env              → Environment variables                  │
│   .KEY            → System and workflow env vars           │
│                                                            │
│ .variables        → User-defined variables                 │
│   .var_name       → Variable values                        │
│                                                            │
│ .tasks            → Task results                           │
│   .task_id        → Task result object                     │
│     .status       → Success/Failed/Skipped                 │
│     .output       → Task output                            │
│     .duration     → Execution time                         │
│     .error        → Error message (if any)                 │
│                                                            │
│ .system           → System information                     │
│   .hostname       → Current hostname                       │
│   .timestamp      → Current timestamp                      │
│   .cwd            → Current working directory              │
└────────────────────────────────────────────────────────────┘

Template Resolution Flow:
  Input: "Output is: {{ .tasks.backup.output }}"
         │
         ▼
  ┌──────────────────┐
  │ Template Engine  │
  │  (Parse + Exec)  │
  └────────┬─────────┘
           │
           ▼
  ┌──────────────────┐
  │ Context Lookup   │
  │  tasks.backup    │
  └────────┬─────────┘
           │
           ▼
  ┌──────────────────┐
  │  Render Value    │
  │  "/backup/file"  │
  └──────────────────┘
```

### Task Registry

Central registry for all available task types.

```
┌──────────────────────────────────────────────────────────┐
│               TASK REGISTRY ARCHITECTURE                  │
└──────────────────────────────────────────────────────────┘

            ┌─────────────────────┐
            │   Task Registry     │
            │   (executors map)   │
            └──────────┬──────────┘
                       │
    ┌──────────────────┼──────────────────┐
    │                  │                  │
    ▼                  ▼                  ▼
┌─────────┐      ┌──────────┐      ┌──────────┐
│ command │      │   file   │      │   ssh    │
│ shell   │      │ template │      │  remote  │
│ script  │      └──────────┘      └──────────┘
└─────────┘            │                  │
    │                  │                  │
    ▼                  ▼                  ▼
┌─────────┐      ┌──────────┐      ┌──────────┐
│compress │      │   copy   │      │  email   │
│ archive │      │          │      │   mail   │
│unarchive│      └──────────┘      └──────────┘
└─────────┘            │                  │
    │                  │                  │
    ▼                  ▼                  ▼
┌─────────┐      ┌──────────┐      ┌──────────┐
│checksum │      │   slack  │      │   ses    │
│  hash   │      │  notify  │      │aws_email │
│ verify  │      └──────────┘      └──────────┘
└─────────┘            │
    │                  │
    ▼                  ▼
┌─────────┐      ┌──────────┐
│  debug  │      │   (more) │
│   log   │      │          │
└─────────┘      └──────────┘

Task Executor Interface:
  type TaskExecutor interface {
    Execute(ctx, task, contextMgr) → TaskResult
    Validate(task) → error
  }

Registration Flow:
  1. Registry.New() creates registry
  2. RegisterBuiltinTasks() adds all built-in types
  3. Registry.RegisterToExecutor(exec) passes to Executor
  4. Executor.RegisterTask(type, executor) stores in map
  5. Runtime: Executor looks up by task.Type

Aliases:
  • "shell" → command executor
  • "template" → file executor
  • "archive" → compress executor
  • etc.
```

---

## Supporting Systems

### Filesystem Factory

Creates Afero filesystems for different storage backends.

```
┌──────────────────────────────────────────────────────────────┐
│              FILESYSTEM FACTORY ARCHITECTURE                  │
└──────────────────────────────────────────────────────────────┘

Input: URI/Path String
  │
  ▼
┌───────────────┐
│  ParsePath()  │
│  • Scheme     │
│  • Host       │
│  • Path       │
│  • Bucket     │
└───────┬───────┘
        │
        ▼
  ┌────────────┐
  │ CreateFS() │
  └─────┬──────┘
        │
        ├────────┬────────┬────────┬────────┐
        ▼        ▼        ▼        ▼        ▼
   ┌────────┐ ┌─────┐ ┌──────┐ ┌──────┐ ┌──────┐
   │  file  │ │ s3  │ │ sftp │ │ ssh  │ │ http │
   └────┬───┘ └──┬──┘ └───┬──┘ └───┬──┘ └───┬──┘
        │        │        │        │        │
        ▼        ▼        ▼        ▼        ▼
  ┌──────────────────────────────────────────────┐
  │           afero.Fs Interface                 │
  │  • Open(path)                                │
  │  • Create(path)                              │
  │  • Stat(path)                                │
  │  • Remove(path)                              │
  │  • ReadDir(path)                             │
  └──────────────────────────────────────────────┘

Supported Schemes:
  • file://     → afero.OsFs (local)
  • s3://       → afero-s3 (AWS S3)
  • sftp://     → SftpFs wrapper (SSH file transfer)
  • ssh://      → SftpFs wrapper (alias)
  • http://     → HttpFs (read-only)
  • https://    → HttpFs (read-only)
  • (no scheme) → Defaults to local file://

Example URIs:
  /tmp/data              → Local filesystem
  file:///tmp/data       → Local filesystem (explicit)
  s3://bucket/path       → AWS S3
  sftp://user@host/path  → SFTP over SSH
  https://example.com/   → HTTP(S) fetch

Cross-Filesystem Operations:
  copy task supports copying between different filesystems:
    src: s3://source-bucket/file.txt
    dest: sftp://backup-server/files/file.txt
```

### Template Engine

Renders Go templates with Sprig functions.

```
┌──────────────────────────────────────────────────────────┐
│            TEMPLATE ENGINE ARCHITECTURE                   │
└──────────────────────────────────────────────────────────┘

                ┌─────────────────┐
                │ Template Engine │
                └────────┬────────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
        ▼                ▼                ▼
  ┌──────────┐    ┌───────────┐    ┌──────────┐
  │   Go     │    │   Sprig   │    │  Custom  │
  │ template │    │ Functions │    │ Functions│
  └──────────┘    └───────────┘    └──────────┘

Template Syntax:
  {{ .variable }}           → Variable lookup
  {{ .tasks.id.output }}    → Task result access
  {{ .env.HOME }}           → Environment variable
  {{ if .condition }}...    → Conditionals
  {{ range .items }}...     → Loops
  {{ default "val" .var }}  → Sprig: default value
  {{ upper .name }}         → Sprig: uppercase
  {{ now | date "2006" }}   → Sprig: date formatting

Available Sprig Functions:
  • String: upper, lower, trim, replace, split, join
  • Math: add, sub, mul, div, mod, max, min
  • Date: now, date, dateModify, duration
  • Encoding: b64enc, b64dec, sha256sum
  • Lists: list, append, prepend, first, last, rest
  • Dicts: dict, get, set, keys, values
  • Logic: default, empty, coalesce, ternary
  • UUID: uuidv4
  • And many more...

Rendering Flow:
  "Hello {{ .variables.name }}"
         │
         ▼
  Parse Template (cached)
         │
         ▼
  Context Data Injection
         │
         ▼
  Execute with Context
         │
         ▼
  "Hello World"
```

### Import Resolver

Resolves and loads workflow imports from multiple sources.

```
┌──────────────────────────────────────────────────────────┐
│            IMPORT RESOLVER ARCHITECTURE                   │
└──────────────────────────────────────────────────────────┘

Workflow with imports:
┌──────────────────────┐
│ imports:             │
│   - ./local.yaml     │
│   - s3://wf.yaml     │
│   - http://wf.yaml   │
└──────────┬───────────┘
           │
           ▼
    ┌──────────────┐
    │   Resolver   │
    └──────┬───────┘
           │
    ┌──────┴──────┬──────────┬────────────┐
    ▼             ▼          ▼            ▼
┌────────┐   ┌────────┐  ┌──────┐   ┌─────────┐
│ Local  │   │   S3   │  │ HTTP │   │   Git   │
│  File  │   │ Bucket │  │ URL  │   │  Repo   │
└───┬────┘   └───┬────┘  └───┬──┘   └────┬────┘
    │            │           │           │
    └────────────┴───────────┴───────────┘
                     │
                     ▼
           ┌─────────────────┐
           │  Parse Imported │
           │    Workflow     │
           └────────┬────────┘
                    │
                    ▼
           ┌─────────────────┐
           │ Merge Tasks &   │
           │   Variables     │
           └────────┬────────┘
                    │
                    ▼
           ┌─────────────────┐
           │ Cycle Detection │
           │  (Max Depth)    │
           └─────────────────┘

Import Resolution Rules:
  • Recursive imports supported (max depth configurable)
  • Circular import detection
  • Variables from parent override imported
  • Tasks are merged (no override, must have unique IDs)
  • Import order matters for variable precedence
```

### Event Systems

Multiple event sources can trigger workflow execution.

```
┌──────────────────────────────────────────────────────────────┐
│                EVENT SYSTEM ARCHITECTURE                      │
└──────────────────────────────────────────────────────────────┘

Event Sources:
                        ┌───────────────┐
                        │ Orchestrator  │
                        └───────┬───────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌──────────────┐       ┌────────────────┐      ┌──────────────┐
│   Webhook    │       │  File Watcher  │      │   Pub/Sub    │
│   Server     │       │                │      │   Listener   │
└──────┬───────┘       └────────┬───────┘      └──────┬───────┘
       │                        │                     │
       │                        │                     │
       ▼                        ▼                     ▼
┌──────────────────────────────────────────────────────────────┐
│                  EVENT HANDLERS                              │
└──────────────────────────────────────────────────────────────┘

Webhook Server Flow:
  HTTP POST → /webhook/:workflow
     │
     ▼
  Parse JSON Payload
     │
     ▼
  Lookup Workflow File
     │
     ▼
  Inject Variables from Payload
     │
     ▼
  Trigger Orchestrator.Run()
     │
     ▼
  Return Execution ID (async)

File Watcher Flow:
  Monitor Directory
     │
     ▼
  File Created/Modified
     │
     ▼
  Match Pattern/Workflow
     │
     ▼
  Read File Metadata
     │
     ▼
  Trigger Workflow with File Path

Pub/Sub Flow:
  Subscribe to Topic
     │
     ▼
  Message Received
     │
     ▼
  Parse Message (JSON)
     │
     ▼
  Extract Workflow Name
     │
     ▼
  Trigger with Message Data
```

---

## Execution Flow

### Complete Workflow Execution Sequence

```
┌──────────────────────────────────────────────────────────────┐
│              COMPLETE EXECUTION FLOW                          │
└──────────────────────────────────────────────────────────────┘

 1. User Input
    │
    ▼
    ritual run workflow.yaml
    │
    ▼
 2. CLI Initialization
    ├─ Load config (.ritual.yaml)
    ├─ Initialize logger
    └─ Parse flags (dry-run, verbose, etc.)
    │
    ▼
 3. Orchestrator Creation
    ├─ Create Parser
    ├─ Create Resolver
    ├─ Create Context Manager
    ├─ Create Executor
    ├─ Create Task Registry
    ├─ Create Import Resolver
    └─ Register all tasks
    │
    ▼
 4. Workflow Loading
    ├─ Parse YAML → Workflow AST
    ├─ Resolve imports (recursive)
    ├─ Merge imported workflows
    └─ Validate syntax
    │
    ▼
 5. Context Initialization
    ├─ Load workflow variables
    ├─ Load environment variables
    ├─ Inject system variables (hostname, timestamp)
    └─ Prepare template engine
    │
    ▼
 6. Dependency Resolution
    ├─ Build dependency graph
    ├─ Validate dependencies exist
    ├─ Detect circular dependencies
    ├─ Perform topological sort (Kahn's algorithm)
    └─ Generate execution layers
    │
    ▼
 7. Pre-Execution Validation
    ├─ Validate all task types exist
    ├─ Validate task configurations
    └─ Check filesystem access
    │
    ▼
 8. Execution (Layer by Layer)
    │
    ├─ Layer 0 (No dependencies)
    │  ├─ Task A │ Task C │ Task D  (parallel)
    │  │    │    │    │    │    │
    │  │    ▼    │    ▼    │    ▼
    │  │  Render │ Execute │ Capture
    │  │  Config │   Task  │ Result
    │  │    │    │    │    │    │
    │  └────┴─────────┴─────────┘
    │       Wait for all to complete
    │
    ├─ Layer 1 (Depends on Layer 0)
    │  ├─ Task B │ Task E  (parallel)
    │  │    │    │    │
    │  │    ▼    │    ▼
    │  │  Execute with previous results
    │  │    │    │    │
    │  └────┴─────────┘
    │
    └─ Layer N (Final)
       ├─ Task F
       │    │
       └────┘
    │
    ▼
 9. Result Collection
    ├─ Aggregate task results
    ├─ Calculate durations
    ├─ Determine workflow status
    └─ Format output (JSON/Text)
    │
    ▼
10. Post-Execution
    ├─ Write history record
    ├─ Send notifications (if configured)
    ├─ Cleanup resources
    └─ Return exit code

Error Handling:
  • Task failure → Mark as failed, continue other tasks
  • Required task fails → Stop layer, abort workflow
  • Context cancelled → Stop all tasks gracefully
  • Panic recovery → Capture, log, mark task failed
```

---

## Data Flow

### Variable and Result Propagation

```
┌──────────────────────────────────────────────────────────────┐
│                  DATA FLOW DIAGRAM                            │
└──────────────────────────────────────────────────────────────┘

Workflow Variables → Context Manager
                           │
                           ▼
      ┌────────────────────────────────────┐
      │        Context Data Store          │
      │                                    │
      │  variables: { key: val }           │
      │  env: { HOME: "/home/user" }       │
      │  tasks: { }  ← (empty initially)   │
      │  workflow: { name, version }       │
      │  system: { hostname, timestamp }   │
      └───────────────┬────────────────────┘
                      │
         ┌────────────┼────────────┐
         │            │            │
         ▼            ▼            ▼
    ┌────────┐  ┌────────┐  ┌────────┐
    │ Task A │  │ Task B │  │ Task C │
    └────┬───┘  └────┬───┘  └────┬───┘
         │           │           │
         │    ┌──────┴───────┐   │
         │    │              │   │
         │    ▼              ▼   │
         │  Template      Template│
         │  Resolution   Resolution
         │    │              │   │
         │    ▼              ▼   │
         │  Execute       Execute│
         │    │              │   │
         │    ▼              ▼   ▼
         │  ┌──────────────────────┐
         │  │   Task Results       │
         │  ├──────────────────────┤
         │  │ task_a: {            │
         │  │   status: "success"  │
         │  │   output: "data"     │
         │  │   duration: "1.2s"   │
         │  │ }                    │
         │  │ task_b: { ... }      │
         │  └──────────┬───────────┘
         │             │
         └─────────────┼─────────────┐
                       │             │
                       ▼             ▼
              Later tasks can reference:
              {{ .tasks.task_a.output }}
              {{ .tasks.task_b.status }}

Example:
  Task A (backup):
    output: "/backup/file-20240102.tar.gz"
    │
    ▼ (stored in context)
    │
  Task B (upload):
    config:
      source: "{{ .tasks.backup.output }}"
               │
               ▼ (template resolved)
               │
      source: "/backup/file-20240102.tar.gz"
```

### Filesystem Data Flow

```
┌──────────────────────────────────────────────────────────────┐
│           FILESYSTEM DATA FLOW                                │
└──────────────────────────────────────────────────────────────┘

Task Config:
  source: "s3://bucket/input.txt"
  dest: "sftp://server/output.txt"
     │
     ▼
┌─────────────────┐
│ Filesystem      │
│ Factory         │
└────┬───────┬────┘
     │       │
     ▼       ▼
┌─────────┐ ┌──────────┐
│  S3 FS  │ │ SFTP FS  │
└────┬────┘ └────┬─────┘
     │           │
     ▼           ▼
┌─────────┐ ┌──────────┐
│  Read   │ │  Write   │
└────┬────┘ └────┬─────┘
     │           │
     └─────┬─────┘
           │
           ▼
    Cross-FS Copy
         │
         ▼
    ┌──────────┐
    │  Result  │
    │  • Bytes │
    │  • Files │
    └──────────┘

Supported Patterns:
  • Local → Local    (standard copy)
  • Local → S3       (upload)
  • S3 → Local       (download)
  • S3 → SFTP        (cloud to remote)
  • HTTP → Local     (fetch)
  • Any → Any        (via afero.Fs interface)
```

---

## Summary

The Ritual workflow engine is designed as a modular, extensible system with clear separation of concerns:

- **CLI** provides the user interface
- **Orchestrator** coordinates all components
- **Resolver** builds and optimizes execution plans
- **Executor** runs tasks with concurrency control
- **Context Manager** handles data flow between tasks
- **Task Registry** provides pluggable task implementations
- **Filesystem Factory** abstracts storage backends
- **Template Engine** enables dynamic configuration

The architecture supports:
- Parallel execution within dependency layers
- Sequential execution between layers
- Multiple filesystem backends
- Event-driven workflow triggering
- Dry-run simulation
- Comprehensive error handling and recovery
- Extensibility through the task executor interface

This design enables Ritual to handle complex workflows with multiple dependencies while maintaining performance, reliability, and ease of use.
