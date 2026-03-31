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
- **CRD types**: `api/<group>/v1beta1/*_types.go` — Spec, Status, and kubebuilder markers (grouped by OCI API group)
- **Controllers**: `controllers/<group>/*_controller.go` — Reconcile loops, largely generated and grouped by OCI API group
- **Service Managers**: `pkg/servicemanager/<group>/<resource>/` — One resource directory per service manager
- **Registration**: `internal/registrations/*_generated.go` + `manual_groups.go` wire schemes/controllers; `main.go` iterates those registrations
- **Formal runtime metadata**: `formal/` — Per-resource runtime intent, provider-fact imports, logic gaps, and generated diagrams for promotion/scaffold tracking
- **CRDs**: `config/crd/` — Generated manifests

### Reference Implementations (use these as templates)

| Pattern | Good Reference |
|---------|----------------|
| Handwritten service logic | `pkg/servicemanager/streams/` |
| Handwritten service logic with secret flow | `pkg/servicemanager/mysql/dbsystem/` |
| Generated scaffold shape | `pkg/servicemanager/core/subnet/` |
| Controller | `controllers/core/subnet_controller.go` |
| CRD Types | `api/streaming/v1beta1/stream_types.go` |

## Conventions & Patterns

### OCI Client Interface (testability)
Every service manager has an injected client interface seam. In older handwritten managers this is usually an `ociClient` field; in generated managers it is usually a `client` field plus `WithClient(...)`:
```go
type FooServiceManager struct {
    Provider  common.ConfigurationProvider
    ociClient FooClientInterface  // handwritten pattern
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

### Service Manager Reality Check
Most service-manager directories are generated scaffolds or placeholders. For actual reconcile logic, prefer `pkg/servicemanager/streams/` and `pkg/servicemanager/mysql/dbsystem/` as the main references.

## Implementing Real Reconciler Logic

When the task is "make a generated resource actually reconcile correctly", assume the main handwritten work belongs in the service-manager package unless the code proves otherwise.

Use this reading order:

1. `api/<group>/v1beta1/<resource>_types.go` — Understand CRD spec/status fields.
2. `controllers/<group>/<resource>_controller.go` — Confirm whether the controller is only BaseReconciler delegation.
3. `internal/registrations/<group>_generated.go` or `manual_groups.go` — Confirm which service manager factory is wired.
4. vendored OCI SDK under `vendor/github.com/oracle/oci-go-sdk/v65/...` — Branch-local source of truth for the current SDK field and operation surface.
5. `pkg/servicemanager/<group>/<resource>/` — Implement the runtime behavior here first.
6. `formal/controller_manifest.tsv` and the matching `formal/controllers/<service>/<slug>/` + `formal/imports/<service>/<slug>.json` — When present, use them to understand intended lifecycle, requeue, delete, secret, and mutation semantics.
7. `oracle/terraform-provider-oci` source — Optional secondary reference for deeper CRUD, wait, datasource, and field-handling details when the local formal summary is not enough.

In generated service-manager packages, look for the handwritten extension seam before rewriting generated files:

- `*_servicemanager.go` is usually a thin adapter
- `*_serviceclient.go` often exposes `WithClient(...)` and the default generated client
- `pkg/servicemanager/generatedruntime/` provides baseline CRUD/status behavior

If a file starts with `// Code generated by generator. DO NOT EDIT.` or `// Code generated by controller-gen. DO NOT EDIT.`, treat it as generator-owned and not a normal handwritten edit target. Do not manually edit those files unless the task is explicitly about generator output or generator source. Prefer editing:

- generator source or source-of-truth config
- manual carve-out files the contract marks as manual
- separate non-generated files in the same package when the generated package exposes an extension seam

Only change controllers or registrations when you need non-default watches, predicates, RBAC, or factory wiring. Otherwise keep the generated wiring intact and implement the real logic in `pkg/servicemanager/<group>/<resource>/`.

Use this priority order when deciding whether a field or operation belongs in scope for the current branch:

1. vendored OCI SDK in this repo
2. checked-in repo contracts and formal metadata
3. Terraform provider as a secondary runtime reference

Do not reject an implementation because a field exists in a newer SDK copy outside this repo unless the task is explicitly about upgrading the pinned SDK or refreshing generated output for that newer version.
