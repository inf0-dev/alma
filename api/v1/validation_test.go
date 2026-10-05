//go:build unit

package v1_test

import (
	"testing"

	v1 "github.com/inf0-dev/alma/api/v1"
	"github.com/inf0-dev/alma/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validDocument returns a minimal valid Document for use as a base in tests.
func validDocument() v1.Document {
	return v1.Document{
		Metadata: v1.Metadata{
			Version: v1.VersionV1,
		},
		Schema: v1.Schema{
			Title: "Test Schema",
			Requirements: []v1.Requirement{
				{ID: "req1", Description: "First requirement", IsHard: true},
			},
			Items: []v1.Item{
				{
					ID:          "q1",
					Description: "Question 1",
					Kind:        v1.KindChoice,
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "yes", Description: "Yes"},
						{ID: "no", Description: "No"},
					},
				},
			},
			DesignOptions: []v1.DesignOption{
				{
					ID:    "opt1",
					Title: "Option 1",
					RequirementsMet: v1.RequirementsMet{
						"req1": v1.RequirementStatus{Met: true},
					},
				},
			},
		},
	}
}

func TestValidDocument(t *testing.T) {
	doc := validDocument()
	require.NoError(t, doc.Validate())
}

func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "invalid version",
			modify: func(d *v1.Document) {
				d.Metadata.Version = "wrong"
			},
			wantErr: "invalid version",
		},
		{
			name: "empty version",
			modify: func(d *v1.Document) {
				d.Metadata.Version = ""
			},
			wantErr: "invalid version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateSchemaFields(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "empty title",
			modify: func(d *v1.Document) {
				d.Schema.Title = ""
			},
			wantErr: "schema title is required",
		},
		{
			name: "invalid issue URL",
			modify: func(d *v1.Document) {
				d.Schema.Issue = "not-a-url"
			},
			wantErr: "error parsing issue URL",
		},
		{
			name: "non-http issue URL scheme",
			modify: func(d *v1.Document) {
				d.Schema.Issue = "ftp://example.com"
			},
			wantErr: "invalid issue URL scheme",
		},
		{
			name: "valid issue URL",
			modify: func(d *v1.Document) {
				d.Schema.Issue = "https://github.com/org/repo/issues/1"
			},
			wantErr: "",
		},
		{
			name: "invalid category name",
			modify: func(d *v1.Document) {
				d.Schema.Categories = []string{"valid", "not valid!"}
			},
			wantErr: "invalid category name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRequirements(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "empty requirement ID",
			modify: func(d *v1.Document) {
				d.Schema.Requirements[0].ID = ""
			},
			wantErr: "requirement ID is required",
		},
		{
			name: "invalid requirement ID characters",
			modify: func(d *v1.Document) {
				d.Schema.Requirements[0].ID = "has spaces"
			},
			wantErr: "requirement ID is required",
		},
		{
			name: "duplicate requirement ID",
			modify: func(d *v1.Document) {
				d.Schema.Requirements = append(d.Schema.Requirements,
					v1.Requirement{ID: "req1", Description: "Duplicate"},
				)
			},
			wantErr: "duplicate requirement ID: req1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateItems(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "empty item ID",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].ID = ""
			},
			wantErr: "item ID is required",
		},
		{
			name: "invalid item ID characters",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].ID = "bad id!"
			},
			wantErr: "item ID is required",
		},
		{
			name: "duplicate item ID",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items,
					v1.Item{ID: "q1", Kind: v1.KindText, Description: "Duplicate"},
				)
			},
			wantErr: "duplicate item ID: q1",
		},
		{
			name: "invalid kind",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].Kind = "banana"
			},
			wantErr: "invalid kind: banana",
		},
		{
			name: "choice options on non-choice item",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:   "q2",
					Kind: v1.KindText,
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "a", Description: "A"},
					},
					Description: "Text with choices",
				})
			},
			wantErr: "has choice options but is not of kind 'choice'",
		},
		{
			name: "empty choice option ID",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].ChoiceOptions[0].ID = ""
			},
			wantErr: "choice option ID is required",
		},
		{
			name: "duplicate choice option ID",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].ChoiceOptions = append(d.Schema.Items[0].ChoiceOptions,
					v1.ChoiceOption{ID: "yes", Description: "Duplicate yes"},
				)
			},
			wantErr: "duplicate choice option ID: yes",
		},
		{
			name: "number config on non-number item",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].NumberConfig = &v1.NumberConfig{Unit: "days"}
			},
			wantErr: "has number config but is not of kind 'number'",
		},
		{
			name: "number config min greater than max",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:           "n1",
					Kind:         v1.KindNumber,
					Description:  "A number",
					NumberConfig: &v1.NumberConfig{Min: testutil.IntPtr(10), Max: testutil.IntPtr(5)},
				})
			},
			wantErr: "has min greater than max",
		},
		{
			name: "valid number config",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:           "n1",
					Kind:         v1.KindNumber,
					Description:  "A number",
					NumberConfig: &v1.NumberConfig{Unit: "months", Min: testutil.IntPtr(1), Max: testutil.IntPtr(12)},
				})
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateDesignOptions(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "no design options",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions = nil
			},
			wantErr: "at least one design option is required",
		},
		{
			name: "empty option ID",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].ID = ""
			},
			wantErr: "design option ID is required",
		},
		{
			name: "duplicate option ID",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions = append(d.Schema.DesignOptions, v1.DesignOption{
					ID:    "opt1",
					Title: "Duplicate",
					RequirementsMet: v1.RequirementsMet{
						"req1": v1.RequirementStatus{Met: true},
					},
				})
			},
			wantErr: "duplicate design option ID: opt1",
		},
		{
			name: "empty option title",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Title = ""
			},
			wantErr: "design option title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateRequirementsMet(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "missing requirement in RequirementsMet",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].RequirementsMet = v1.RequirementsMet{}
			},
			wantErr: "is missing requirement status for req1",
		},
		{
			name: "unknown requirement in RequirementsMet",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].RequirementsMet["ghost"] = v1.RequirementStatus{Met: false}
			},
			wantErr: "references unknown requirement: ghost",
		},
		{
			name: "partial requirement status",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].RequirementsMet["req1"] = v1.RequirementStatus{
					Partial: true,
					Reason:  "almost there",
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateEffects(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "effect with unknown answer key",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Effects = map[string][]v1.Effect{
					"q1.unknown": {{Kind: v1.EffectChanges, Text: "something"}},
				}
			},
			wantErr: "unknown answer key: q1.unknown",
		},
		{
			name: "effect with invalid kind",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Effects = map[string][]v1.Effect{
					"q1.yes": {{Kind: "invalid", Text: "something"}},
				}
			},
			wantErr: "invalid kind: invalid",
		},
		{
			name: "effect with empty text",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Effects = map[string][]v1.Effect{
					"q1.yes": {{Kind: v1.EffectChanges, Text: ""}},
				}
			},
			wantErr: "is missing text",
		},
		{
			name: "effect with unknown category",
			modify: func(d *v1.Document) {
				d.Schema.Categories = []string{"report", "export"}
				d.Schema.DesignOptions[0].Effects = map[string][]v1.Effect{
					"q1.yes": {{Kind: v1.EffectChanges, Category: "ghost", Text: "something"}},
				}
			},
			wantErr: "unknown category: ghost",
		},
		{
			name: "effect with valid category",
			modify: func(d *v1.Document) {
				d.Schema.Categories = []string{"report", "export"}
				d.Schema.DesignOptions[0].Effects = map[string][]v1.Effect{
					"q1.yes": {{Kind: v1.EffectChanges, Category: "report", Text: "something"}},
				}
			},
			wantErr: "",
		},
		{
			name: "valid effect without categories defined",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Effects = map[string][]v1.Effect{
					"q1.yes": {{Kind: v1.EffectNote, Category: "anything", Text: "note about it"}},
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateBlocks(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "block with empty condition",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Blocks = []v1.Block{
					{Condition: []string{}, Reason: "some reason"},
				}
			},
			wantErr: "must have at least one condition",
		},
		{
			name: "block with empty reason",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Blocks = []v1.Block{
					{Condition: []string{"q1.yes"}, Reason: ""},
				}
			},
			wantErr: "is missing a reason",
		},
		{
			name: "block with unknown answer key",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Blocks = []v1.Block{
					{Condition: []string{"q1.yes", "q1.unknown"}, Reason: "breaks"},
				}
			},
			wantErr: "unknown answer key: q1.unknown",
		},
		{
			name: "valid block",
			modify: func(d *v1.Document) {
				d.Schema.DesignOptions[0].Blocks = []v1.Block{
					{Condition: []string{"q1.yes", "q1.no"}, Reason: "contradicts"},
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateShowWhen(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Document)
		wantErr string
	}{
		{
			name: "valid show_when",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:   "q2",
					Kind: v1.KindChoice,
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "a", Description: "A"},
					},
					Description: "Depends on q1",
					ShowWhen:    [][]string{{"q1.yes"}},
				})
			},
			wantErr: "",
		},
		{
			name: "show_when with unknown answer key",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:          "q2",
					Kind:        v1.KindText,
					Description: "Depends on ghost",
					ShowWhen:    [][]string{{"q1.ghost"}},
				})
			},
			wantErr: "unknown answer key: q1.ghost",
		},
		{
			name: "show_when with empty group",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:          "q2",
					Kind:        v1.KindText,
					Description: "Empty group",
					ShowWhen:    [][]string{{}},
				})
			},
			wantErr: "must have at least one condition",
		},
		{
			name: "show_when self-reference",
			modify: func(d *v1.Document) {
				d.Schema.Items[0].ShowWhen = [][]string{{"q1.yes"}}
			},
			wantErr: "references itself",
		},
		{
			name: "show_when circular dependency",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:   "q2",
					Kind: v1.KindChoice,
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "x", Description: "X"},
					},
					Description: "Depends on q1",
					ShowWhen:    [][]string{{"q1.yes"}},
				})
				d.Schema.Items[0].ShowWhen = [][]string{{"q2.x"}}
			},
			wantErr: "circular show_when dependency",
		},
		{
			name: "show_when OR groups valid",
			modify: func(d *v1.Document) {
				d.Schema.Items = append(d.Schema.Items, v1.Item{
					ID:          "q2",
					Kind:        v1.KindText,
					Description: "OR condition",
					ShowWhen:    [][]string{{"q1.yes"}, {"q1.no"}},
				})
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validDocument()
			tt.modify(&doc)
			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateGathersMultipleErrors(t *testing.T) {
	doc := v1.Document{
		Metadata: v1.Metadata{Version: "wrong"},
		Schema: v1.Schema{
			Title:         "",
			DesignOptions: nil,
		},
	}
	err := doc.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid version")
	assert.Contains(t, err.Error(), "schema title is required")
	assert.Contains(t, err.Error(), "at least one design option is required")
	assert.Contains(t, err.Error(), "3 errors")
}

// --- Record validation tests ---

// validRecord returns a minimal valid Record for use as a base in tests.
func validRecord() v1.Record {
	return v1.Record{
		Version:     v1.RecordVersionV1,
		Model:       validDocument(),
		ModelSHA256: "abc123def456",
		DecidedOn:   "2026-10-01",
		Present:     []string{"Alice"},
		Requirements: []v1.RecordRequirement{
			{ID: "req1", IsHard: true, Checked: true},
		},
		Items: []v1.RecordItem{
			{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		},
		Options: []v1.RecordOption{},
		Final: &v1.FinalDecision{
			Option:    "opt1",
			Title:     "Option 1",
			Rationale: "Best fit",
		},
	}
}

func TestValidRecord(t *testing.T) {
	rec := validRecord()
	require.NoError(t, rec.Validate())
}

func TestValidateRecordMetadata(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Record)
		wantErr string
	}{
		{
			name: "invalid record version",
			modify: func(r *v1.Record) {
				r.Version = "wrong"
			},
			wantErr: "invalid record version",
		},
		{
			name: "missing model_sha256",
			modify: func(r *v1.Record) {
				r.ModelSHA256 = ""
			},
			wantErr: "model_sha256 is required",
		},
		{
			name: "missing decided_on",
			modify: func(r *v1.Record) {
				r.DecidedOn = ""
			},
			wantErr: "decided_on is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.modify(&rec)
			err := rec.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateRecordEmbeddedModel(t *testing.T) {
	rec := validRecord()
	rec.Model.Metadata.Version = "bad"
	err := rec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded model")
}

func TestValidateRecordRequirements(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Record)
		wantErr string
	}{
		{
			name: "unknown requirement ID",
			modify: func(r *v1.Record) {
				r.Requirements = []v1.RecordRequirement{
					{ID: "ghost", IsHard: true, Checked: true},
				}
			},
			wantErr: "unknown requirement: ghost",
		},
		{
			name: "missing requirement",
			modify: func(r *v1.Record) {
				r.Requirements = []v1.RecordRequirement{}
			},
			wantErr: "missing requirement: req1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.modify(&rec)
			err := rec.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateRecordItems(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Record)
		wantErr string
	}{
		{
			name: "unknown item ID",
			modify: func(r *v1.Record) {
				r.Items = []v1.RecordItem{
					{ID: "ghost", Kind: v1.KindText},
				}
			},
			wantErr: "unknown item: ghost",
		},
		{
			name: "missing item",
			modify: func(r *v1.Record) {
				r.Items = []v1.RecordItem{}
			},
			wantErr: "missing item: q1",
		},
		{
			name: "kind mismatch",
			modify: func(r *v1.Record) {
				r.Items = []v1.RecordItem{
					{ID: "q1", Kind: v1.KindText},
				}
			},
			wantErr: "has kind text, model has choice",
		},
		{
			name: "unknown choice answer",
			modify: func(r *v1.Record) {
				r.Items = []v1.RecordItem{
					{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("maybe")},
				}
			},
			wantErr: "unknown answer: maybe",
		},
		{
			name: "valid nil answer",
			modify: func(r *v1.Record) {
				r.Items = []v1.RecordItem{
					{ID: "q1", Kind: v1.KindChoice, Answer: nil},
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.modify(&rec)
			err := rec.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRecordFinalDecision(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Record)
		wantErr string
	}{
		{
			name: "unknown final option",
			modify: func(r *v1.Record) {
				r.Final = &v1.FinalDecision{Option: "ghost", Title: "Ghost"}
			},
			wantErr: "unknown option: ghost",
		},
		{
			name: "nil final is valid",
			modify: func(r *v1.Record) {
				r.Final = nil
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.modify(&rec)
			err := rec.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRecordHistory(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*v1.Record)
		wantErr string
	}{
		{
			name: "missing decided_on in history",
			modify: func(r *v1.Record) {
				r.History = []v1.HistoryEntry{
					{Option: "opt1", ModelSHA256: "abc"},
				}
			},
			wantErr: "history[0] is missing decided_on",
		},
		{
			name: "missing option in history",
			modify: func(r *v1.Record) {
				r.History = []v1.HistoryEntry{
					{DecidedOn: "2026-09-01", ModelSHA256: "abc"},
				}
			},
			wantErr: "history[0] is missing option",
		},
		{
			name: "missing model_sha256 in history",
			modify: func(r *v1.Record) {
				r.History = []v1.HistoryEntry{
					{DecidedOn: "2026-09-01", Option: "opt1"},
				}
			},
			wantErr: "history[0] is missing model_sha256",
		},
		{
			name: "valid history entry",
			modify: func(r *v1.Record) {
				r.History = []v1.HistoryEntry{
					{DecidedOn: "2026-09-01", Option: "opt1", Title: "Option 1", ModelSHA256: "old_hash"},
				}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.modify(&rec)
			err := rec.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateRecordGathersMultipleErrors(t *testing.T) {
	rec := v1.Record{
		Version:     "wrong",
		ModelSHA256: "",
		DecidedOn:   "",
		Model: v1.Document{
			Metadata: v1.Metadata{Version: v1.VersionV1},
			Schema: v1.Schema{
				Title:         "Test",
				DesignOptions: []v1.DesignOption{{ID: "opt1", Title: "O1"}},
			},
		},
	}
	err := rec.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid record version")
	assert.Contains(t, err.Error(), "model_sha256 is required")
	assert.Contains(t, err.Error(), "decided_on is required")
}
