package ioformat

import (
	"fmt"
	"strings"
	"time"

	"github.com/Giancarlos/guardrails/internal/models"
)

// ConvertOptions controls how a BeadsIssue is turned into a Task.
type ConvertOptions struct {
	// IncludeComments appends each comment as a timestamped note. Default true.
	IncludeComments bool
	// LabelPrefix prefixes synthesized labels (e.g. "bd:deferred"). Default "bd:".
	LabelPrefix string
	// TypeOverrides maps bd issue_type -> gur type. Takes precedence over the
	// built-in mapping.
	TypeOverrides map[string]string
}

// DefaultConvertOptions returns the standard options used by `gur import`.
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		IncludeComments: true,
		LabelPrefix:     "bd:",
	}
}

// ConvertedTask is the result of mapping one BeadsIssue. `Task.ID` is left
// empty so the import pipeline can assign a fresh gur-* id.
type ConvertedTask struct {
	Task       models.Task
	SourceID   string      // original bd id (e.g. "bd-probe-xtw")
	DepTargets []MappedDep // deps to create once all source IDs are resolved
}

// MappedDep captures a dependency edge in bd-id space. The import pipeline
// resolves both ends to gur-* IDs via the bd->gur map.
type MappedDep struct {
	FromSourceID string // dependent (child, blocked)
	ToSourceID   string // blocker (parent)
	Type         string // gur dep type: blocks, related, parent-child
	OriginalType string // original bd type for diagnostic purposes
}

// ConvertIssue maps one BeadsIssue to a guardrails Task + pending dep edges.
// It does not allocate a gur ID — the caller does that and then rewrites
// DepTargets to concrete IDs.
func ConvertIssue(issue BeadsIssue, opts ConvertOptions) ConvertedTask {
	if opts.LabelPrefix == "" {
		opts.LabelPrefix = "bd:"
	}

	labels := append([]string(nil), issue.Labels...)

	status, statusLabel := mapStatus(issue.Status)
	if statusLabel != "" {
		labels = append(labels, opts.LabelPrefix+statusLabel)
	}

	typ, typeLabel := mapIssueType(issue.IssueType, opts.TypeOverrides)
	if typeLabel != "" {
		labels = append(labels, opts.LabelPrefix+"type:"+typeLabel)
	}

	description := buildDescription(issue)
	notes := buildNotes(issue, opts.IncludeComments)

	assignee := issue.Assignee
	if assignee == "" {
		assignee = issue.Owner
	} else if issue.Owner != "" && issue.Owner != issue.Assignee {
		labels = append(labels, opts.LabelPrefix+"owner:"+issue.Owner)
	}

	sourceID := issue.ID
	task := models.Task{
		Title:       truncate(issue.Title, 255),
		Description: description,
		Status:      status,
		Priority:    clampPriority(issue.Priority),
		Type:        typ,
		Labels:      models.StringSlice(labels),
		Assignee:    assignee,
		Notes:       notes,
		CloseReason: issue.CloseReason,
		Source:      models.SourceBeads,
		SourceID:    &sourceID,
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
		ClosedAt:    issue.ClosedAt,
	}

	var deps []MappedDep
	for _, d := range issue.Dependencies {
		gurType, _ := mapDepType(d.Type)
		deps = append(deps, MappedDep{
			FromSourceID: d.IssueID,
			ToSourceID:   d.DependsOnID,
			Type:         gurType,
			OriginalType: d.Type,
		})
	}

	return ConvertedTask{Task: task, SourceID: sourceID, DepTargets: deps}
}

// mapStatus returns (gurStatus, extraLabel). extraLabel is empty when the
// mapping is exact; it carries the original bd status when we had to
// collapse.
func mapStatus(s string) (string, string) {
	switch s {
	case "open", "":
		return models.StatusOpen, ""
	case "in_progress":
		return models.StatusInProgress, ""
	case "closed":
		return models.StatusClosed, ""
	case "blocked":
		return models.StatusOpen, "" // blocked is implied by deps in gur
	case "deferred":
		return models.StatusOpen, "deferred"
	case "pinned":
		return models.StatusOpen, "pinned"
	case "hooked":
		return models.StatusInProgress, "hooked"
	default:
		return models.StatusOpen, s
	}
}

// mapIssueType returns (gurType, originalIfCollapsed). The overrides map (if
// non-nil) takes precedence.
func mapIssueType(t string, overrides map[string]string) (string, string) {
	if v, ok := overrides[t]; ok {
		return v, ""
	}
	switch t {
	case "task", "":
		return models.TypeTask, ""
	case "bug":
		return models.TypeBug, ""
	case "feature":
		return models.TypeFeature, ""
	case "epic":
		return models.TypeEpic, ""
	default:
		// chore/decision/spike/story/milestone/<custom>
		return models.TypeTask, t
	}
}

// mapDepType collapses bd's 10 dep types into gur's 3.
// Returns (gurType, originalIfCollapsed).
func mapDepType(t string) (string, string) {
	switch t {
	case "blocks", "":
		return models.DepTypeBlocks, ""
	case "parent-child":
		return models.DepTypeParentChild, ""
	case "related", "relates-to":
		return models.DepTypeRelated, ""
	default:
		// tracks, discovered-from, until, caused-by, validates, supersedes
		return models.DepTypeRelated, t
	}
}

func clampPriority(p int) int {
	if p < models.PriorityCritical {
		return models.PriorityCritical
	}
	if p > models.PriorityLowest {
		return models.PriorityLowest
	}
	return p
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// buildDescription merges bd's description + design + acceptance_criteria
// into a single markdown string. bd keeps them separate; gur has one column.
func buildDescription(issue BeadsIssue) string {
	var b strings.Builder
	if issue.Description != "" {
		b.WriteString(issue.Description)
	}
	if issue.Design != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## Design\n\n")
		b.WriteString(issue.Design)
	}
	if issue.AcceptanceCriteria != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## Acceptance Criteria\n\n")
		b.WriteString(issue.AcceptanceCriteria)
	}
	return b.String()
}

// buildNotes combines bd notes + comments + a trailer with fields that have
// no native gur slot (estimated_minutes, due_at, defer_until, external_ref).
func buildNotes(issue BeadsIssue, includeComments bool) string {
	var b strings.Builder
	if issue.Notes != "" {
		b.WriteString(issue.Notes)
	}

	if includeComments {
		for _, c := range issue.Comments {
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
				b.WriteString("\n")
			}
			ts := c.CreatedAt.Format(models.DateTimeFormat)
			fmt.Fprintf(&b, "[%s] %s: %s\n", ts, c.Author, c.Text)
		}
	}

	trailer := buildBdTrailer(issue)
	if trailer != "" {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString(trailer)
	}

	return b.String()
}

// buildBdTrailer emits fields that have no direct gur column as an HTML
// comment so round-trip export can recover them.
func buildBdTrailer(issue BeadsIssue) string {
	var parts []string
	if issue.EstimatedMinutes > 0 {
		parts = append(parts, fmt.Sprintf("estimated_minutes=%d", issue.EstimatedMinutes))
	}
	if issue.DueAt != nil {
		parts = append(parts, "due_at="+issue.DueAt.Format(time.RFC3339))
	}
	if issue.DeferUntil != nil {
		parts = append(parts, "defer_until="+issue.DeferUntil.Format(time.RFC3339))
	}
	if issue.ExternalRef != "" {
		parts = append(parts, "external_ref="+issue.ExternalRef)
	}
	if len(parts) == 0 {
		return ""
	}
	return "<!-- bd: " + strings.Join(parts, "; ") + " -->"
}
