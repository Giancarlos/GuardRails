package ioformat

import (
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Giancarlos/guardrails/internal/models"
)

// ExportTask is the on-wire shape of one task for gur's native JSONL export.
// It's structurally similar to BeadsIssue but uses gur's vocabulary.
type ExportTask struct {
	models.Task
	Dependencies []ExportDep `json:"dependencies,omitempty"`
}

// ExportDep is the on-wire shape of a dependency edge.
type ExportDep struct {
	ParentID string `json:"parent_id"` // blocker
	ChildID  string `json:"child_id"`  // blocked (owner of the record)
	Type     string `json:"type"`
}

// EncodeGurJSONL writes tasks as one-JSON-per-line. Each task carries its
// outgoing dependencies (child perspective) so the line is self-contained.
func EncodeGurJSONL(w io.Writer, tasks []models.Task, depsByChild map[string][]models.Dependency) error {
	enc := json.NewEncoder(w)
	for _, t := range tasks {
		var eds []ExportDep
		for _, d := range depsByChild[t.ID] {
			eds = append(eds, ExportDep{ParentID: d.ParentID, ChildID: d.ChildID, Type: d.Type})
		}
		if err := enc.Encode(ExportTask{Task: t, Dependencies: eds}); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBeadsJSONL writes tasks in bd 1.0.x-compatible JSONL. Lossy on:
// archived status (→ closed + label), hierarchical subtask IDs (flattened to
// parent-child deps — the hierarchy already lives in tasks.parent_id).
//
// Dep refs are rewritten back to source_id where available so round-tripped
// exports look natural to bd tooling; tasks without a source_id keep their
// gur-* id.
func EncodeBeadsJSONL(w io.Writer, tasks []models.Task, depsByChild map[string][]models.Dependency) error {
	idMap := make(map[string]string, len(tasks))
	for _, t := range tasks {
		if t.SourceID != nil && *t.SourceID != "" {
			idMap[t.ID] = *t.SourceID
		} else {
			idMap[t.ID] = t.ID
		}
	}

	enc := json.NewEncoder(w)
	for _, t := range tasks {
		issue := taskToBeads(t, depsByChild[t.ID], idMap)
		if err := enc.Encode(issue); err != nil {
			return err
		}
	}
	return nil
}

var bdTrailerRE = regexp.MustCompile(`(?s)\n?<!-- bd: (.*?) -->\s*$`)

func taskToBeads(t models.Task, deps []models.Dependency, idMap map[string]string) BeadsIssue {
	status := t.Status
	labels := append([]string(nil), t.Labels...)
	if status == models.StatusArchived {
		status = "closed"
		labels = append(labels, "archived")
	}

	desc, design, acceptance := splitDescription(t.Description)
	notes, trailer := splitNotesTrailer(t.Notes)

	issue := BeadsIssue{
		// For beads-jsonl export, we use the gur ID as-is. If the task was
		// imported from bd, the original id lives in t.SourceID — prefer that.
		ID:                 preferSourceID(t),
		Title:              t.Title,
		Description:        desc,
		Design:             design,
		AcceptanceCriteria: acceptance,
		Notes:              notes,
		Status:             status,
		Priority:           t.Priority,
		IssueType:          mapTypeToBeads(t.Type),
		Assignee:           t.Assignee,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
		ClosedAt:           t.ClosedAt,
		CloseReason:        t.CloseReason,
		Labels:             labels,
	}

	applyTrailer(&issue, trailer)

	for _, d := range deps {
		issue.Dependencies = append(issue.Dependencies, BeadsDependency{
			IssueID:     resolveExportID(d.ChildID, idMap),
			DependsOnID: resolveExportID(d.ParentID, idMap),
			Type:        mapDepTypeToBeads(d.Type),
		})
	}

	return issue
}

func resolveExportID(gurID string, idMap map[string]string) string {
	if v, ok := idMap[gurID]; ok {
		return v
	}
	return gurID
}

func preferSourceID(t models.Task) string {
	if t.SourceID != nil && *t.SourceID != "" {
		return *t.SourceID
	}
	return t.ID
}

func mapTypeToBeads(t string) string {
	switch t {
	case models.TypeBug:
		return "bug"
	case models.TypeFeature:
		return "feature"
	case models.TypeEpic:
		return "epic"
	default:
		return "task"
	}
}

func mapDepTypeToBeads(t string) string {
	switch t {
	case models.DepTypeParentChild:
		return "parent-child"
	case models.DepTypeRelated:
		return "related"
	default:
		return "blocks"
	}
}

// splitDescription reverses buildDescription: extracts optional ## Design and
// ## Acceptance Criteria sections when they sit at the end.
func splitDescription(s string) (desc, design, acceptance string) {
	desc = s
	if idx := strings.LastIndex(desc, "\n\n## Acceptance Criteria\n\n"); idx >= 0 {
		acceptance = desc[idx+len("\n\n## Acceptance Criteria\n\n"):]
		desc = desc[:idx]
	} else if strings.HasPrefix(desc, "## Acceptance Criteria\n\n") {
		acceptance = desc[len("## Acceptance Criteria\n\n"):]
		desc = ""
	}
	if idx := strings.LastIndex(desc, "\n\n## Design\n\n"); idx >= 0 {
		design = desc[idx+len("\n\n## Design\n\n"):]
		desc = desc[:idx]
	} else if strings.HasPrefix(desc, "## Design\n\n") {
		design = desc[len("## Design\n\n"):]
		desc = ""
	}
	return
}

// splitNotesTrailer returns (notes, trailer) where trailer is the raw content
// inside `<!-- bd: ... -->` if present at end of notes.
func splitNotesTrailer(notes string) (string, string) {
	m := bdTrailerRE.FindStringSubmatchIndex(notes)
	if m == nil {
		return notes, ""
	}
	trailer := notes[m[2]:m[3]]
	return strings.TrimRight(notes[:m[0]], "\n"), trailer
}

func applyTrailer(issue *BeadsIssue, trailer string) {
	if trailer == "" {
		return
	}
	for _, part := range strings.Split(trailer, ";") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "estimated_minutes":
			if n, err := strconv.Atoi(v); err == nil {
				issue.EstimatedMinutes = n
			}
		case "due_at":
			if ts, err := time.Parse(time.RFC3339, v); err == nil {
				issue.DueAt = &ts
			}
		case "defer_until":
			if ts, err := time.Parse(time.RFC3339, v); err == nil {
				issue.DeferUntil = &ts
			}
		case "external_ref":
			issue.ExternalRef = v
		}
	}
}
