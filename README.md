# GuardRails

A command-line task management tool for AI agents. Built with Go and SQLite.

## Features

- Task management with priorities, types, and labels
- Task dependencies and blockers
- Quality gates (tests, reviews, approvals)
- Subtask hierarchies
- Reusable task templates
- Change history/audit trail
- JSON output for automation

## Installation

```bash
go build -o gur .
```

Or use the Makefile:

```bash
make build
```

## Quick Start

```bash
# Initialize in current directory
gur init

# Create a task
gur create "My first task"

# List tasks
gur list

# Show task details
gur show <id>

# Close a task
gur close <id>
```

## Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize GuardRails in current directory |
| `create` | Create a new task |
| `list` | List tasks with optional filters |
| `show` | Display task details |
| `update` | Modify a task |
| `close` | Close a task |
| `reopen` | Reopen a closed task |
| `ready` | Show tasks with no open blockers |
| `dep` | Manage task dependencies |
| `gate` | Manage quality gates |
| `template` | Manage task templates |
| `search` | Search tasks |
| `stats` | Show project statistics |
| `history` | View change audit trail |
| `archive` | Archive completed tasks |
| `compact` | Compress old task data |

## Dependencies

Quality gates can be linked to tasks to prevent closure until gates pass:

```bash
# Create a gate
gur gate create "Unit tests"

# Link gate to task
gur gate link <gate-id> <task-id>

# Record gate result
gur gate pass <gate-id>
```

## Task Dependencies

```bash
# Add a blocking dependency
gur dep add <blocker-id> <blocked-id>

# View dependencies
gur dep list <task-id>
```

## License

MIT License - see [LICENSE.md](LICENSE.md)
