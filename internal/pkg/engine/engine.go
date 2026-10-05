package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/inf0-dev/alma/api/v1"
)

// Evaluate computes derived fields (option statuses, meets counts, fired blocks/effects, still_open)
// from a document and the current session state (requirements checked, items answered).
func Evaluate(doc *v1.Document, requirements []v1.RecordRequirement, items []v1.RecordItem) (*v1.Record, error) {
	hash, err := computeSHA256(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to compute SHA256: %w", err)
	}

	// Seed requirements from schema if none provided
	if requirements == nil {
		requirements = make([]v1.RecordRequirement, 0, len(doc.Schema.Requirements))
		for _, r := range doc.Schema.Requirements {
			requirements = append(requirements, v1.RecordRequirement{
				ID:      r.ID,
				IsHard:  r.IsHard,
				Checked: false,
			})
		}
	}

	// Seed items from schema if none provided
	if items == nil {
		items = make([]v1.RecordItem, 0, len(doc.Schema.Items))
		for _, item := range doc.Schema.Items {
			ri := v1.RecordItem{
				ID:   item.ID,
				Kind: item.Kind,
			}
			if item.Kind == v1.KindNumber && item.NumberConfig != nil {
				ri.Unit = item.NumberConfig.Unit
			}
			items = append(items, ri)
		}
	}

	checkedReqs := make(map[string]bool)
	for _, r := range requirements {
		checkedReqs[r.ID] = r.Checked
	}

	hardReqs := make(map[string]bool)
	for _, r := range doc.Schema.Requirements {
		hardReqs[r.ID] = r.IsHard
	}

	// Build show_when lookup from schema
	showWhen := make(map[string][][]string)
	for _, si := range doc.Schema.Items {
		if len(si.ShowWhen) > 0 {
			showWhen[si.ID] = si.ShowWhen
		}
	}

	// Compute visible items iteratively (cascade: hiding an item clears its
	// answer, which may hide further items that depend on it).
	visibleItems := computeVisibleItems(items, showWhen)

	// Set Visible on each item and build active answer keys from visible items only
	for i := range items {
		v := visibleItems[items[i].ID]
		items[i].Visible = &v
	}

	activeAnswerKeys := make(map[string]struct{})
	for _, item := range items {
		if item.Kind == v1.KindChoice && item.Answer != nil && visibleItems[item.ID] {
			activeAnswerKeys[item.ID+"."+*item.Answer] = struct{}{}
		}
	}

	// Evaluate each design option
	options := make([]v1.RecordOption, 0, len(doc.Schema.DesignOptions))
	for _, opt := range doc.Schema.DesignOptions {
		recOpt := evaluateOption(opt, checkedReqs, hardReqs, activeAnswerKeys)
		options = append(options, recOpt)
	}

	// Compute still_open: visible choice items with no answer
	var stillOpen []string
	for _, item := range items {
		if item.Kind == v1.KindChoice && item.Answer == nil && visibleItems[item.ID] {
			stillOpen = append(stillOpen, item.ID)
		}
	}

	return &v1.Record{
		Version:      v1.RecordVersionV1,
		Model:        *doc,
		ModelSHA256:  hash,
		Requirements: requirements,
		Items:        items,
		Options:      options,
		StillOpen:    stillOpen,
	}, nil
}

func evaluateOption(
	opt v1.DesignOption,
	checkedReqs map[string]bool,
	hardReqs map[string]bool,
	activeAnswerKeys map[string]struct{},
) v1.RecordOption {
	recOpt := v1.RecordOption{ID: opt.ID}

	// Compute meets counts and failed requirements
	var hardMet, hardOf, softMet, softOf int
	var failedReqs []string

	for reqID, status := range opt.RequirementsMet {
		isHard := hardReqs[reqID]
		checked := checkedReqs[reqID]

		if isHard {
			hardOf++
			if status.Met || status.Partial {
				hardMet++
			} else if checked {
				failedReqs = append(failedReqs, reqID)
			}
		} else {
			softOf++
			if status.Met || status.Partial {
				softMet++
			}
		}
	}

	recOpt.Meets.Hard = v1.MeetsCounts{Met: hardMet, Of: hardOf}
	recOpt.Meets.Soft = v1.MeetsCounts{Met: softMet, Of: softOf}
	if failedReqs == nil {
		failedReqs = []string{}
	}
	recOpt.FailedRequirements = failedReqs

	// Check blocks: AND within a block, OR across blocks
	var firedBlocks []string
	for _, block := range opt.Blocks {
		allMatch := true
		for _, key := range block.Condition {
			if _, active := activeAnswerKeys[key]; !active {
				allMatch = false
				break
			}
		}
		if allMatch {
			firedBlocks = append(firedBlocks, block.Reason)
		}
	}
	if firedBlocks == nil {
		firedBlocks = []string{}
	}
	recOpt.FiredBlocks = firedBlocks

	// Collect fired effects for chosen answers
	var effects []v1.FiredEffect
	for key, effs := range opt.Effects {
		if _, active := activeAnswerKeys[key]; !active {
			continue
		}
		parts := strings.SplitN(key, ".", 2)
		for _, eff := range effs {
			effects = append(effects, v1.FiredEffect{
				Item:     parts[0],
				Answer:   parts[1],
				Kind:     eff.Kind,
				Category: eff.Category,
				Text:     eff.Text,
			})
		}
	}
	recOpt.Effects = effects

	// Determine status: eliminated > blocked > possible
	if len(failedReqs) > 0 {
		recOpt.Status = v1.StatusEliminated
	} else if len(firedBlocks) > 0 {
		recOpt.Status = v1.StatusBlocked
	} else {
		recOpt.Status = v1.StatusPossible
	}

	return recOpt
}

// computeVisibleItems determines which items are visible given current answers and
// show_when conditions. Uses iterative fixpoint: an item is visible if it has no
// show_when, or at least one OR-group is fully satisfied by visible, answered items.
// Cascades: if hiding item A causes item B's conditions to fail, B hides too.
func computeVisibleItems(items []v1.RecordItem, showWhen map[string][][]string) map[string]bool {
	visible := make(map[string]bool, len(items))
	// Start: all items without show_when are visible
	for _, item := range items {
		if _, ok := showWhen[item.ID]; !ok {
			visible[item.ID] = true
		}
	}

	// Build answer lookup
	answers := make(map[string]string) // item_id -> answer_id
	for _, item := range items {
		if item.Kind == v1.KindChoice && item.Answer != nil {
			answers[item.ID] = *item.Answer
		}
	}

	for {
		changed := false
		for itemID, groups := range showWhen {
			wasVisible := visible[itemID]
			nowVisible := false
			for _, group := range groups {
				allMet := true
				for _, key := range group {
					parts := strings.SplitN(key, ".", 2)
					if len(parts) != 2 {
						allMet = false
						break
					}
					depID, ansID := parts[0], parts[1]
					if !visible[depID] || answers[depID] != ansID {
						allMet = false
						break
					}
				}
				if allMet {
					nowVisible = true
					break
				}
			}
			if nowVisible != wasVisible {
				visible[itemID] = nowVisible
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	return visible
}

// Export finalizes the current record. If a previous record is provided (non-nil),
// its Final decision is moved into the current record's History. Returns a new Record.
func Export(prev *v1.Record, curr *v1.Record) *v1.Record {
	out := *curr
	out.DecidedOn = time.Now().UTC().Format(time.RFC3339)

	if prev != nil {
		out.History = make([]v1.HistoryEntry, len(prev.History))
		copy(out.History, prev.History)

		if prev.Final != nil {
			out.History = append(out.History, v1.HistoryEntry{
				DecidedOn:   prev.DecidedOn,
				Present:     prev.Present,
				Option:      prev.Final.Option,
				Title:       prev.Final.Title,
				Rationale:   prev.Final.Rationale,
				ModelSHA256: prev.ModelSHA256,
			})
		}
	}

	return &out
}

// computeSHA256 computes the SHA256 hash of the given document and returns it as a hexadecimal string.
func computeSHA256(doc *v1.Document) (string, error) {
	docBytes, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(docBytes)), nil
}
