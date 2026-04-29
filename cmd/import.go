package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/Giancarlos/guardrails/internal/db"
	"github.com/Giancarlos/guardrails/internal/ioformat"
	"github.com/Giancarlos/guardrails/internal/models"
)

var (
	importFormat      string
	importDryRun      bool
	importOnConflict  string
	importNoComments  bool
	importTypeMaps    []string
	importLabelPrefix string
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import tasks from an external format",
	Long: `Import tasks into guardrails from an external format.

Currently supports beads 1.0.x JSONL (the output of 'bd export').

Idempotency: tasks are matched on their original bd id (stored in the
source_id column). Default --on-conflict=update upserts existing rows.

Non-issue records (memories, agents, rigs, roles, messages) are skipped
with a summary warning at the end.`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVar(&importFormat, "format", "beads-jsonl", "Input format (beads-jsonl)")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Parse + validate, print summary, no writes")
	importCmd.Flags().StringVar(&importOnConflict, "on-conflict", "update", "Behavior when source_id already exists: update, skip, error")
	importCmd.Flags().BoolVar(&importNoComments, "no-comments", false, "Do not fold comments into notes")
	importCmd.Flags().StringArrayVar(&importTypeMaps, "map-type", nil, "Override type mapping, e.g. --map-type spike=bug (repeatable)")
	importCmd.Flags().StringVar(&importLabelPrefix, "label-prefix", "bd:", "Prefix for synthesized labels")
}

func runImport(cmd *cobra.Command, args []string) error {
	if importFormat != "beads-jsonl" {
		return fmt.Errorf("unsupported --format %q (only beads-jsonl is supported)", importFormat)
	}
	switch importOnConflict {
	case "update", "skip", "error":
	default:
		return fmt.Errorf("invalid --on-conflict %q (want update, skip, or error)", importOnConflict)
	}

	typeOverrides, err := parseTypeMaps(importTypeMaps)
	if err != nil {
		return err
	}

	f, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	res, err := ioformat.DecodeBeadsJSONL(f)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	opts := ioformat.DefaultConvertOptions()
	opts.IncludeComments = !importNoComments
	opts.LabelPrefix = importLabelPrefix
	opts.TypeOverrides = typeOverrides

	converted := make([]ioformat.ConvertedTask, 0, len(res.Issues))
	for _, iss := range res.Issues {
		converted = append(converted, ioformat.ConvertIssue(iss, opts))
	}

	summary := map[string]any{
		"dry_run":          importDryRun,
		"file":             args[0],
		"issues":           len(converted),
		"memories_skipped": res.MemoriesSkipped,
		"infra_skipped":    res.InfraSkipped,
		"invalid_lines":    res.InvalidLines,
	}

	if importDryRun {
		printImportSummary(summary, 0, 0, nil)
		return nil
	}

	inserted, updated, unknownDeps, err := applyImport(converted, importOnConflict)
	if err != nil {
		return err
	}

	summary["inserted"] = inserted
	summary["updated"] = updated
	summary["unknown_dep_targets"] = len(unknownDeps)
	printImportSummary(summary, inserted, updated, unknownDeps)
	return nil
}

func parseTypeMaps(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	m := make(map[string]string, len(raw))
	for _, s := range raw {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" || v == "" {
			return nil, fmt.Errorf("invalid --map-type %q: want bd_type=gur_type", s)
		}
		m[k] = v
	}
	return m, nil
}

// applyImport runs the whole insert/upsert in a single transaction.
// Returns (inserted, updated, unknownDepTargets, error).
func applyImport(converted []ioformat.ConvertedTask, onConflict string) (int, int, []string, error) {
	database := db.GetDB()
	var inserted, updated int
	var unknownDeps []string

	err := database.Transaction(func(tx *gorm.DB) error {
		bdToGur := make(map[string]string, len(converted))

		for i := range converted {
			ct := &converted[i]
			existing, found, err := findBySourceID(tx, ct.SourceID)
			if err != nil {
				return err
			}
			if found {
				if onConflict == "error" {
					return fmt.Errorf("task with source_id %q already exists (id=%s); use --on-conflict=update or skip", ct.SourceID, existing.ID)
				}
				bdToGur[ct.SourceID] = existing.ID
				if onConflict == "skip" {
					continue
				}
				ct.Task.ID = existing.ID
				if err := tx.Session(&gorm.Session{FullSaveAssociations: false}).
					Select("*").Omit("CreatedAt").Save(&ct.Task).Error; err != nil {
					return fmt.Errorf("update %s: %w", ct.SourceID, err)
				}
				updated++
				continue
			}

			ct.Task.ID = models.GenerateID()
			if err := tx.Create(&ct.Task).Error; err != nil {
				return fmt.Errorf("insert %s: %w", ct.SourceID, err)
			}
			bdToGur[ct.SourceID] = ct.Task.ID
			inserted++
		}

		// Pass 2: dependencies, once every source id has a gur id.
		for _, ct := range converted {
			for _, d := range ct.DepTargets {
				from, okF := bdToGur[d.FromSourceID]
				to, okT := bdToGur[d.ToSourceID]
				if !okF || !okT {
					unknownDeps = append(unknownDeps, fmt.Sprintf("%s->%s (%s)", d.FromSourceID, d.ToSourceID, d.Type))
					continue
				}
				var existing int64
				if err := tx.Model(&models.Dependency{}).
					Where("parent_id = ? AND child_id = ? AND type = ?", to, from, d.Type).
					Count(&existing).Error; err != nil {
					return fmt.Errorf("dep lookup %s->%s: %w", from, to, err)
				}
				if existing > 0 {
					continue
				}
				dep := models.Dependency{ParentID: to, ChildID: from, Type: d.Type}
				if err := tx.Create(&dep).Error; err != nil {
					return fmt.Errorf("dep %s->%s: %w", from, to, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, nil, err
	}
	return inserted, updated, unknownDeps, nil
}

func findBySourceID(tx *gorm.DB, sid string) (*models.Task, bool, error) {
	var t models.Task
	err := tx.Where("source_id = ?", sid).First(&t).Error
	if err == nil {
		return &t, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, false, nil
	}
	return nil, false, err
}

func printImportSummary(summary map[string]any, inserted, updated int, unknownDeps []string) {
	if IsJSONOutput() {
		if unknownDeps != nil {
			summary["unknown_deps"] = unknownDeps
		}
		OutputJSON(summary)
		return
	}

	fmt.Printf("imported from %s: %d issue(s)\n", summary["file"], summary["issues"])
	if !importDryRun {
		fmt.Printf("  inserted: %d, updated: %d\n", inserted, updated)
	} else {
		fmt.Println("  (dry run — no writes)")
	}

	if n, _ := summary["memories_skipped"].(int); n > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: dropped %d beads memories (guardrails has no memory store)\n", n)
	}
	if n, _ := summary["infra_skipped"].(int); n > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: skipped %d infrastructure/unknown records (agents, rigs, roles, messages, custom types)\n", n)
	}
	if n, _ := summary["invalid_lines"].(int); n > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %d lines failed to parse\n", n)
	}
	if len(unknownDeps) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: %d dependencies reference unknown tasks and were skipped:\n", len(unknownDeps))
		for _, d := range unknownDeps {
			fmt.Fprintf(os.Stderr, "  %s\n", d)
		}
	}
}
