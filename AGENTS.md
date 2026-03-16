# Project Instructions for AI Agents

This file provides instructions and context for AI coding agents working on this project.

<!-- BEGIN BEADS INTEGRATION -->
## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**

```bash
bd ready --json
```

**Create new issues:**

```bash
bd create "Issue title" --description="Detailed context" -t bug|feature|task -p 0-4 --json
bd create "Issue title" --description="What this issue is about" -p 1 --deps discovered-from:bd-123 --json
```

**Claim and update:**

```bash
bd update <id> --claim --json
bd update bd-42 --priority 1 --json
```

**Complete work:**

```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task atomically**: `bd update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems

<!-- END BEADS INTEGRATION -->

## Landing the Plane (Session Completion)

**When ending a work session**, complete these steps:

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Commit all changes** - Ensure everything is committed locally
5. **Hand off** - Provide context for next session

**NOTE**: Do NOT push to remote — the human handles `git push`.

<!-- BEGIN MULTI-AGENT SETUP -->
## Multi-Agent Workflow

This project supports two agent architectures, selected via `launch.sh`:

| Mode | Command | Planner | Panes |
|------|---------|---------|-------|
| **Default (single)** | `./agnts/launch.sh` | Single planner agent | 3 panes |
| **Multi-agent** | `./agnts/launch.sh multi` | Planner orchestrator with draft + review sub-agents | 3 panes |

### Agents

| Role | Tmux Pane | Responsibility |
|------|-----------|----------------|
| **Planner** | pane 0 | Designs tasks and creates beads |
| **Coder** | pane 1 | Picks ready tasks, implements code, submits for review. NEVER creates tasks. |
| **Reviewer** | pane 2 | Reviews coder output, approves or returns with feedback. NEVER writes code. |

In **multi-agent mode**, the planner spawns two sub-agents internally:

| Sub-agent | Config | Responsibility |
|-----------|--------|----------------|
| **planner_draft** | `.codex/agents/planner_draft.toml` | Designs task decompositions, writes to `agnts/plans/draft.md`. NEVER writes to beads. |
| **planner_review** | `.codex/agents/planner_review.toml` | Reviews plans, **ONLY** agent that runs `bd create`/`bd dep add` (single-writer rule). |

### Launching Agents

```bash
# Default mode (single planner)
./agnts/launch.sh                  # Launch all 3 agents
./agnts/launch.sh planner          # Launch only planner (single)
./agnts/launch.sh coder            # Launch only coder
./agnts/launch.sh reviewer         # Launch only reviewer

# Multi-agent mode (planner with sub-agents)
./agnts/launch.sh multi            # Launch all 3 agents (multi-agent planner)
./agnts/launch.sh planner-multi    # Launch only planner orchestrator
./agnts/launch.sh planner_draft    # Launch only planner_draft (standalone)
./agnts/launch.sh planner_review   # Launch only planner_review (standalone)

# Override profile:
CODEX_PROFILE=gpt-5-4 ./agnts/launch.sh
```

### Role Instructions

Each agent reads its role-specific instructions at startup:
- `agnts/roles/planner-single.md` — Single planner (default mode)
- `agnts/roles/planner.md` — Multi-agent orchestrator (multi mode)
- `agnts/roles/planner_draft.md` — Design and task decomposition (writes plan file)
- `agnts/roles/planner_review.md` — Plan review and beads creation (single writer)
- `agnts/roles/coder.md` — Implementation workflow
- `agnts/roles/reviewer.md` — Review checklist and feedback protocol

### Review Status Convention

Tasks flow through these statuses:

```
open → in_progress (claimed by coder) → in_progress + needs-review label → closed (approved)
                                              ↓
                                         open (rejected, returned with feedback)
```

- **Coder marks for review**: `bd label add <id> needs-review` (status stays `in_progress`)
- **Reviewer approves**: `bd label remove <id> needs-review` + `bd close <id> --reason="Approved"`
- **Reviewer rejects**: `bd comments add <id> "REVIEW FEEDBACK: ..."` + `bd label remove <id> needs-review` + `bd update <id> --status=open` (keeps assignee so coder finds it via `--assignee="$(git config user.name)"`)

### Planning Label Gate

When the planner creates an epic, it adds a `planning` label to prevent the coder from grabbing tasks before all dependencies are wired:
- **Planner adds**: `bd label add <epic-id> planning` (right after creating the epic)
- **Planner removes**: `bd label remove <epic-id> planning` (after all tasks + deps are wired)
- **Coder checks**: Before claiming, verify the parent epic does NOT have `planning` label

### Coordination Rules

- **Task creation**: ONLY the planner (or planner_review in multi mode) creates beads
- **Task claiming**: ONLY coders claim tasks via `bd update <id> --claim`
- **Code review**: ONLY the reviewer approves or rejects code
- **Git push**: ONLY the human pushes to remote
- **Dependency ordering**: Coders must use `bd ready` — never pick blocked tasks manually
- **Feedback loop**: Rejected tasks go back to `open` status with review comments; coder picks them up via `bd ready`

### Context Management

- **Each agent runs in its own context window** — token costs scale linearly with agent count
- **Coder**: One task per focused context is ideal. Restart session between complex tasks for fresh context.
- **Reviewer**: Can review multiple tasks per session (reviews are lighter on context).
- **Planner**: One planning session per epic. Restart if creating a new unrelated epic.
- **Signal ~60% context**: When context is getting heavy, finish current task, commit, and restart.
- **Only use multi-agent when parallelism helps**: For simple sequential tasks, a single agent is more token-efficient.
<!-- END MULTI-AGENT SETUP -->

## Build & Test

```bash
go build ./...          # Build — must pass before any commit
go test ./...           # Tests
go vet ./...            # Static analysis
make generate           # Regenerate deepcopy (after modifying *_types.go)
make manifests          # Regenerate CRD YAML (after modifying *_types.go)
```

**CRITICAL**: Always commit ALL generated files (`zz_generated.deepcopy.go`, CRD YAML).

## Architecture Overview

- **Language**: Go 1.25 | **Framework**: kubebuilder / controller-runtime v0.17 | **SDK**: `oci-go-sdk/v65`
- **Manager entry points**: `cmd/manager/<service>/main.go` — per-service binaries; each service builds its own image and manifest set
- **Shared bootstrap**: `pkg/manager/run.go` — common controller-runtime manager setup used by every service binary
- **Service registration**: `pkg/manager/services/<service>.go` — registers only that service's controllers/webhooks with `manager.Run`
- **CRD types**: `api/<service>/v1beta1/*_types.go` — Spec, Status, and kubebuilder markers (grouped by OCI service)
- **Controllers**: `controllers/<service>/*_controller.go` — Reconcile loops (grouped by OCI service, 3 currently)
- **Service Managers**: `pkg/servicemanager/*/` — OCI API client wrappers (3 currently)
- **Service manifests**: `config/manager/<service>/` and `dist/packages/<service>/` — per-service deployment packaging
- **CRDs**: `config/crd/` — Generated manifests

### Reference Implementations (use these as templates)

| Pattern | Good Reference |
|---------|----------------|
| Simple service | `pkg/servicemanager/streams/` |
| Complex service | `pkg/servicemanager/autonomousdatabases/adb/` |
| Controller | `controllers/streaming/stream_controller.go` |
| CRD Types | `api/streaming/v1beta1/stream_types.go` |

## Conventions & Patterns

### OCI Client Interface (testability)
Every service manager has an injected OCI client interface field:
```go
type FooServiceManager struct {
    Provider  common.ConfigurationProvider
    ociClient FooClientInterface  // nil = create from Provider
}
```

### Lifecycle State Handling
Always handle non-terminal states with a requeue:
```go
if instance.LifecycleState == "FAILED" {
    // set failed status, return false
} else if instance.LifecycleState == "ACTIVE" {
    // set active status, return true
} else {
    // set provisioning status, requeue
}
```

### Conditional OCI Fields
Never send zero-value optional fields:
```go
if spec.Port != 0 { details.Port = common.Int(spec.Port) }
if spec.Description != "" { details.Description = common.String(spec.Description) }
```

### Secret Generation
After resource is ACTIVE, write endpoint/connection info to a k8s Secret. See `stream_secretgeneration.go` or `dbsystem_secretgeneration.go` for the pattern.
