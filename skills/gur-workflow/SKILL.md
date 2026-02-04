---
description: "Task management patterns and best practices for the gur CLI tool"
version: "1.0"
---

# gur Workflow Skill

This skill teaches effective use of `gur`, a task management CLI designed for AI agent workflows.

## Core Concepts

### Task Lifecycle
```
open -> in_progress -> closed -> archived
```

- **open**: Task is ready to be worked on
- **in_progress**: Actively being worked on
- **closed**: Completed with a close reason
- **archived**: Historical record, can be compacted to save context

### Priority Levels
- **P0 (Critical)**: Drop everything, fix now
- **P1 (High)**: Important, address soon
- **P2 (Medium)**: Normal priority (default)
- **P3 (Low)**: Nice to have
- **P4 (Lowest)**: Backlog

### Task Types
- **task**: General work item (default)
- **bug**: Defect to fix
- **feature**: New functionality
- **epic**: Large initiative containing subtasks

## Command Patterns

### Creating Tasks

Basic task:
```bash
gur create "Implement user authentication"
```

With full context:
```bash
gur create "Fix login timeout bug" \
  --type bug \
  --priority 1 \
  --label security \
  --label auth \
  --assignee agent
```

From template:
```bash
gur create "Add payment processing" --template feature-checklist
```

### Working With Subtasks

Create parent epic:
```bash
gur create "User management system" --type epic
# Returns: gur-a1b2c3d4
```

Add subtasks:
```bash
gur create "Design user schema" --parent gur-a1b2c3d4
gur create "Implement CRUD endpoints" --parent gur-a1b2c3d4
gur create "Add validation" --parent gur-a1b2c3d4
```

Subtasks get hierarchical IDs: `gur-a1b2c3d4.1`, `gur-a1b2c3d4.2`, etc.

### Managing Dependencies

Block a task until another completes:
```bash
# "auth" blocks "dashboard" (dashboard can't close until auth is done)
gur dep add gur-auth123 gur-dash456
```

Check what's blocking:
```bash
gur show gur-dash456  # Shows "Blocked by: gur-auth123"
```

Find unblocked tasks ready for work:
```bash
gur ready
```

### Quality Gates

Gates enforce checks before task closure.

Create a gate:
```bash
gur gate create "Unit tests pass" --type test --command "go test ./..."
gur gate create "Code review approved" --type review
gur gate create "Security scan clean" --type approval --priority 0
```

Link gate to task:
```bash
gur gate link gate-abc123 gur-task456
```

Record gate results:
```bash
gur gate pass gate-abc123 --by agent --notes "All 42 tests passing"
gur gate fail gate-abc123 --by agent --notes "3 tests failing in auth module"
```

Attempt to close (will check gates):
```bash
gur close gur-task456 "Implementation complete"
# Error: Cannot close - failing gates: "Unit tests pass"
```

### Task Updates

Update status:
```bash
gur update gur-abc123 --status in_progress
```

Add notes (timestamped):
```bash
gur update gur-abc123 --note "Found root cause - null pointer in line 42"
```

Change priority:
```bash
gur update gur-abc123 --priority 0  # Escalate to critical
```

### Searching and Filtering

Search by keyword:
```bash
gur search "authentication"
```

List with filters:
```bash
gur list --status open --priority 0,1  # Critical and high priority
gur list --type bug --assignee agent
gur list --label security
```

### Closing Tasks

Simple close:
```bash
gur close gur-abc123 "Fixed by commit abc123"
```

Force close (skip gate checks):
```bash
gur close gur-abc123 "Closing as won't fix" --force
```

Reopen if needed:
```bash
gur reopen gur-abc123
```

### Linking Skills and Agents

Associate domain knowledge:
```bash
gur update gur-abc123 --skill security-audit --skill go-testing
```

Assign agents:
```bash
gur update gur-abc123 --agent Explore --agent Plan
```

### History and Audit

View task history:
```bash
gur history gur-abc123
```

Archive old closed tasks:
```bash
gur archive --before 30d
```

Compact to save context:
```bash
gur compact --all  # Summarize and clear verbose fields
```

### Statistics

View project stats:
```bash
gur stats
```

Session summary:
```bash
gur summary
```

## Best Practices

### 1. Break Down Large Tasks

Instead of one massive task, create an epic with subtasks:

```bash
# Bad: One huge task
gur create "Build entire payment system"

# Good: Epic with focused subtasks
gur create "Payment system" --type epic
gur create "Payment model and migrations" --parent gur-xxx
gur create "Stripe integration" --parent gur-xxx
gur create "Payment API endpoints" --parent gur-xxx
gur create "Payment UI components" --parent gur-xxx
```

### 2. Use Dependencies for Ordering

When tasks must be done in sequence:

```bash
gur create "Design API schema"      # gur-design
gur create "Implement API"          # gur-impl
gur create "Write API tests"        # gur-tests

gur dep add gur-design gur-impl     # impl waits for design
gur dep add gur-impl gur-tests      # tests wait for impl
```

Then `gur ready` shows only unblocked tasks.

### 3. Use Gates for Quality Checks

For tasks requiring verification:

```bash
gur gate create "Tests pass" --type test
gur gate create "Reviewed" --type review
gur gate link gate-tests gur-feature
gur gate link gate-review gur-feature
```

This prevents premature closure.

### 4. Add Context in Notes

Document discoveries and decisions:

```bash
gur update gur-abc123 --note "Investigated: issue is in the cache layer, not DB"
gur update gur-abc123 --note "Decision: using Redis instead of Memcached"
```

### 5. Use Labels for Cross-Cutting Concerns

```bash
gur create "Fix XSS vulnerability" --label security --label frontend
gur create "Add CSRF protection" --label security --label backend
gur list --label security  # Find all security tasks
```

### 6. Close with Meaningful Reasons

```bash
# Good close reasons
gur close gur-abc123 "Fixed in commit a1b2c3d - added null check"
gur close gur-def456 "Won't fix - feature deprecated in v2"
gur close gur-ghi789 "Duplicate of gur-xyz000"

# Bad close reasons
gur close gur-abc123 "Done"  # No context
```

### 7. Archive and Compact Regularly

Keep context window clean:

```bash
# Archive tasks closed more than 30 days ago
gur archive --before 30d

# Compact archived tasks to minimal form
gur compact --all
```

### 8. Use JSON Output for Automation

```bash
gur list --json | jq '.[] | select(.priority <= 1)'
gur show gur-abc123 --json
```

## Common Workflows

### Bug Triage

```bash
# Create bug with details
gur create "App crashes on login" --type bug --priority 1 --label crash

# Investigate and update
gur update gur-xxx --status in_progress
gur update gur-xxx --note "Reproduced: happens with special chars in password"

# Fix and close
gur close gur-xxx "Fixed in commit abc - escaped special characters"
```

### Feature Development

```bash
# Plan as epic
gur create "Add dark mode" --type feature --type epic

# Break into subtasks
gur create "Add theme context" --parent gur-xxx
gur create "Create dark color palette" --parent gur-xxx
gur create "Add theme toggle component" --parent gur-xxx
gur create "Persist theme preference" --parent gur-xxx

# Add quality gates
gur gate create "Dark mode tests" --type test
gur gate link gate-xxx gur-xxx

# Work through subtasks
gur ready  # See what's unblocked
```

### Code Review Workflow

```bash
# Create gate for reviews
gur gate create "PR approved" --type review

# Link to task
gur gate link gate-review gur-feature

# After review
gur gate pass gate-review --by "reviewer" --notes "LGTM, approved"

# Now task can be closed
gur close gur-feature "Merged in PR #123"
```

## GitHub Integration

### Setup

```bash
gur config github
# Enter: owner/repo
# Enter: GitHub token (stored in system keyring)
```

### Sync to GitHub Issues

```bash
# Push all tasks as issues
gur sync push --all

# Push only open tasks
gur sync push --open

# Dry run first
gur sync push --all --dry-run
```

Tasks sync bidirectionally - GitHub issue numbers are tracked for updates.
