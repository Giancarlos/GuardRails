# Project Instructions

This is the **guardrails** project, which contains the `gur` CLI tool for task management.

## Task Tracking

**Always use `gur` (the binary in the project root) for task tracking instead of the TodoWrite tool.**

The `gur` CLI is specifically designed for this project and provides:
- Persistent task storage in a SQLite database
- Hierarchical subtasks, dependencies, and quality gates
- GitHub Issues sync
- Skills and agents linking

### Quick Reference

```bash
# List open tasks
./gur list --status open

# List in-progress tasks
./gur list --status in_progress

# Show task details
./gur show <task-id>

# Create a new task
./gur create "Task title" --type task --priority 2

# Update task status
./gur update <task-id> --status in_progress

# Close a task
./gur close <task-id> "Reason for closing"

# Find tasks ready to work on (no blockers)
./gur ready
```

### Workflow

1. Before starting work, run `./gur list --status open` or `./gur ready` to see available tasks
2. When starting a task, update its status: `./gur update <id> --status in_progress`
3. Add notes as you work: `./gur update <id> --note "Found the issue in..."`
4. When done, close with a reason: `./gur close <id> "Fixed by implementing..."`

### Full Documentation

See `skills/gur-workflow/SKILL.md` for complete documentation on:
- Task lifecycle and priorities
- Subtasks and dependencies
- Quality gates
- GitHub sync
- Best practices
