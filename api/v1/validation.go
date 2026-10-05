package v1

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var validString = regexp.MustCompile(`^[a-zA-Z0-9_\-/]+$`)

const validStringMessage = "must only contain alphanumeric characters, underscores, hyphens, and slashes"

// Validate checks the validity of the document and returns all validation errors found.
func (s *Document) Validate() error {
	var errs []error

	md := s.Metadata
	if md.Version != VersionV1 {
		errs = append(errs, fmt.Errorf("invalid version: expected %s, got %s", VersionV1, md.Version))
	}

	schema := s.Schema
	if schema.Title == "" {
		errs = append(errs, fmt.Errorf("schema title is required"))
	}

	// Validate the issue URL if it's provided
	if schema.Issue != "" {
		if u, err := url.ParseRequestURI(schema.Issue); err != nil {
			errs = append(errs, fmt.Errorf("error parsing issue URL: %s", schema.Issue))
		} else if u.Scheme != "http" && u.Scheme != "https" {
			errs = append(errs, fmt.Errorf("invalid issue URL scheme: %s", u.Scheme))
		}
	}

	categorySet := make(map[string]struct{})
	for _, cat := range schema.Categories {
		if !validString.MatchString(cat) {
			errs = append(errs, fmt.Errorf("invalid category name: %s. %s", cat, validStringMessage))
		}
		categorySet[cat] = struct{}{}
	}

	requirementIDs := make(map[string]struct{})
	for _, req := range schema.Requirements {
		if req.ID == "" || !validString.MatchString(req.ID) {
			errs = append(errs, fmt.Errorf("requirement ID is required, and must match: %s", validStringMessage))
			continue
		}
		if _, exists := requirementIDs[req.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate requirement ID: %s", req.ID))
		}
		requirementIDs[req.ID] = struct{}{}
	}

	itemIDs := make(map[string]struct{})
	// validAnswerKeys holds all valid "<item_id>.<answer_id>" for cross-referencing effects and blocks.
	validAnswerKeys := make(map[string]struct{})
	for _, item := range schema.Items {
		if item.ID == "" || !validString.MatchString(item.ID) {
			errs = append(errs, fmt.Errorf("item ID is required, and must match: %s", validStringMessage))
			continue
		}
		if _, exists := itemIDs[item.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate item ID: %s", item.ID))
		}
		itemIDs[item.ID] = struct{}{}

		switch item.Kind {
		case KindChoice, KindText, KindNumber:
		default:
			errs = append(errs, fmt.Errorf("item %s has invalid kind: %s", item.ID, item.Kind))
		}

		if len(item.ChoiceOptions) != 0 {
			if item.Kind != KindChoice {
				errs = append(errs, fmt.Errorf("item %s has choice options but is not of kind 'choice'", item.ID))
			}

			choiceIDs := make(map[string]struct{})
			for _, choice := range item.ChoiceOptions {
				if choice.ID == "" || !validString.MatchString(choice.ID) {
					errs = append(errs, fmt.Errorf("choice option ID is required, and must match: %s for item %s. %s", choice.ID, item.ID, validStringMessage))
					continue
				}
				if _, exists := choiceIDs[choice.ID]; exists {
					errs = append(errs, fmt.Errorf("duplicate choice option ID: %s for item %s", choice.ID, item.ID))
				}
				choiceIDs[choice.ID] = struct{}{}
				validAnswerKeys[item.ID+"."+choice.ID] = struct{}{}
			}
		}

		if item.NumberConfig != nil {
			if item.Kind != KindNumber {
				errs = append(errs, fmt.Errorf("item %s has number config but is not of kind 'number'", item.ID))
			}

			if item.NumberConfig.Min != nil && item.NumberConfig.Max != nil && *item.NumberConfig.Min > *item.NumberConfig.Max {
				errs = append(errs, fmt.Errorf("item %s has min greater than max in number config", item.ID))
			}
		}

		for i, group := range item.ShowWhen {
			if len(group) == 0 {
				errs = append(errs, fmt.Errorf("item %s show_when[%d] must have at least one condition", item.ID, i))
			}
			for _, key := range group {
				if _, exists := validAnswerKeys[key]; !exists {
					errs = append(errs, fmt.Errorf("item %s show_when[%d] references unknown answer key: %s", item.ID, i, key))
				}
				// Ensure the item does not reference itself
				parts := strings.SplitN(key, ".", 2)
				if len(parts) == 2 && parts[0] == item.ID {
					errs = append(errs, fmt.Errorf("item %s show_when[%d] references itself", item.ID, i))
				}
			}
		}
	}

	// Detect circular show_when dependencies
	if err := detectShowWhenCycles(schema.Items); err != nil {
		errs = append(errs, err)
	}

	if len(schema.DesignOptions) == 0 {
		errs = append(errs, fmt.Errorf("at least one design option is required"))
	}

	optionIDs := make(map[string]struct{})
	for _, option := range schema.DesignOptions {
		if option.ID == "" || !validString.MatchString(option.ID) {
			errs = append(errs, fmt.Errorf("design option ID is required, and must match: %s", validStringMessage))
			continue
		}
		if _, exists := optionIDs[option.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate design option ID: %s", option.ID))
		}
		optionIDs[option.ID] = struct{}{}

		if option.Title == "" {
			errs = append(errs, fmt.Errorf("design option title is required for option %s", option.ID))
		}

		// Every requirement must be present in RequirementsMet
		for reqID := range requirementIDs {
			if _, exists := option.RequirementsMet[reqID]; !exists {
				errs = append(errs, fmt.Errorf("design option %s is missing requirement status for %s", option.ID, reqID))
			}
		}
		// RequirementsMet keys must reference existing requirements
		for reqID := range option.RequirementsMet {
			if _, exists := requirementIDs[reqID]; !exists {
				errs = append(errs, fmt.Errorf("design option %s references unknown requirement: %s", option.ID, reqID))
			}
		}

		// Effects keys must be valid "<item_id>.<answer_id>" references
		for key, effects := range option.Effects {
			if _, exists := validAnswerKeys[key]; !exists {
				errs = append(errs, fmt.Errorf("design option %s has effect for unknown answer key: %s", option.ID, key))
			}
			for i, effect := range effects {
				if effect.Kind != EffectChanges && effect.Kind != EffectNote {
					errs = append(errs, fmt.Errorf("design option %s effect[%d] for %s has invalid kind: %s", option.ID, i, key, effect.Kind))
				}
				if effect.Text == "" {
					errs = append(errs, fmt.Errorf("design option %s effect[%d] for %s is missing text", option.ID, i, key))
				}
				if len(schema.Categories) > 0 && effect.Category != "" {
					if _, exists := categorySet[effect.Category]; !exists {
						errs = append(errs, fmt.Errorf("design option %s effect[%d] for %s references unknown category: %s", option.ID, i, key, effect.Category))
					}
				}
			}
		}

		// Blocks: each condition entry must be a valid answer key
		for i, block := range option.Blocks {
			if len(block.Condition) == 0 {
				errs = append(errs, fmt.Errorf("design option %s block[%d] must have at least one condition", option.ID, i))
			}
			if block.Reason == "" {
				errs = append(errs, fmt.Errorf("design option %s block[%d] is missing a reason", option.ID, i))
			}
			for _, key := range block.Condition {
				if _, exists := validAnswerKeys[key]; !exists {
					errs = append(errs, fmt.Errorf("design option %s block[%d] references unknown answer key: %s", option.ID, i, key))
				}
			}
		}
	}

	return formatErrors(errs)
}

// Validate checks the validity of the record and returns all validation errors found.
// Derived fields (Options, StillOpen) are not validated as they are recomputed on import.
func (r *Record) Validate() error {
	var errs []error

	if r.Version != RecordVersionV1 {
		errs = append(errs, fmt.Errorf("invalid record version: expected %s, got %s", RecordVersionV1, r.Version))
	}

	// Validate the embedded model
	if err := r.Model.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("embedded model: %w", err))
	}

	if r.ModelSHA256 == "" {
		errs = append(errs, fmt.Errorf("model_sha256 is required"))
	}

	if r.DecidedOn == "" {
		errs = append(errs, fmt.Errorf("decided_on is required"))
	}

	// Build lookup sets from the embedded model
	modelReqIDs := make(map[string]struct{})
	for _, req := range r.Model.Schema.Requirements {
		modelReqIDs[req.ID] = struct{}{}
	}

	modelItemIDs := make(map[string]Kind)
	choiceAnswerIDs := make(map[string]map[string]struct{})
	for _, item := range r.Model.Schema.Items {
		modelItemIDs[item.ID] = item.Kind
		if item.Kind == KindChoice {
			answers := make(map[string]struct{})
			for _, co := range item.ChoiceOptions {
				answers[co.ID] = struct{}{}
			}
			choiceAnswerIDs[item.ID] = answers
		}
	}

	modelOptionIDs := make(map[string]struct{})
	for _, opt := range r.Model.Schema.DesignOptions {
		modelOptionIDs[opt.ID] = struct{}{}
	}

	// Validate requirements match the model
	recordReqIDs := make(map[string]struct{})
	for i, req := range r.Requirements {
		if _, exists := modelReqIDs[req.ID]; !exists {
			errs = append(errs, fmt.Errorf("record requirement[%d] references unknown requirement: %s", i, req.ID))
		}
		recordReqIDs[req.ID] = struct{}{}
	}
	for reqID := range modelReqIDs {
		if _, exists := recordReqIDs[reqID]; !exists {
			errs = append(errs, fmt.Errorf("record is missing requirement: %s", reqID))
		}
	}

	// Validate items match the model
	recordItemIDs := make(map[string]struct{})
	for i, item := range r.Items {
		modelKind, exists := modelItemIDs[item.ID]
		if !exists {
			errs = append(errs, fmt.Errorf("record item[%d] references unknown item: %s", i, item.ID))
			continue
		}
		recordItemIDs[item.ID] = struct{}{}

		if item.Kind != modelKind {
			errs = append(errs, fmt.Errorf("record item %s has kind %s, model has %s", item.ID, item.Kind, modelKind))
		}

		// Validate choice answers reference valid options
		if item.Answer != nil && modelKind == KindChoice {
			if answers, ok := choiceAnswerIDs[item.ID]; ok {
				if _, valid := answers[*item.Answer]; !valid {
					errs = append(errs, fmt.Errorf("record item %s has unknown answer: %s", item.ID, *item.Answer))
				}
			}
		}
	}
	for itemID := range modelItemIDs {
		if _, exists := recordItemIDs[itemID]; !exists {
			errs = append(errs, fmt.Errorf("record is missing item: %s", itemID))
		}
	}

	// Validate final decision references a real option
	if r.Final != nil {
		if _, exists := modelOptionIDs[r.Final.Option]; !exists {
			errs = append(errs, fmt.Errorf("final decision references unknown option: %s", r.Final.Option))
		}
	}

	// Validate history entries
	for i, entry := range r.History {
		if entry.DecidedOn == "" {
			errs = append(errs, fmt.Errorf("history[%d] is missing decided_on", i))
		}
		if entry.Option == "" {
			errs = append(errs, fmt.Errorf("history[%d] is missing option", i))
		}
		if entry.ModelSHA256 == "" {
			errs = append(errs, fmt.Errorf("history[%d] is missing model_sha256", i))
		}
	}

	return formatErrors(errs)
}

// detectShowWhenCycles checks for circular dependencies in show_when references.
// An item's show_when can reference other items; if A->B->A, neither can ever become visible.
func detectShowWhenCycles(items []Item) error {
	// Build dependency graph: item ID → set of item IDs it depends on
	deps := make(map[string]map[string]struct{})
	for _, item := range items {
		if len(item.ShowWhen) == 0 {
			continue
		}
		depSet := make(map[string]struct{})
		for _, group := range item.ShowWhen {
			for _, key := range group {
				parts := strings.SplitN(key, ".", 2)
				if len(parts) == 2 {
					depSet[parts[0]] = struct{}{}
				}
			}
		}
		deps[item.ID] = depSet
	}

	// DFS cycle detection
	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // done
	)
	color := make(map[string]int)

	var visit func(id string) error
	visit = func(id string) error {
		color[id] = gray
		for dep := range deps[id] {
			if color[dep] == gray {
				return fmt.Errorf("circular show_when dependency: %s and %s depend on each other", id, dep)
			}
			if color[dep] == white {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}

	for id := range deps {
		if color[id] == white {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}

// formatErrors returns nil if no errors, or a numbered error list.
func formatErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "validation failed (%d errors):", len(errs))
	for i, err := range errs {
		fmt.Fprintf(&b, "\n  %d. %s", i+1, err.Error())
	}
	return errors.New(b.String())
}
