//go:build unit

package engine_test

import (
	"testing"
	"time"

	v1 "github.com/inf0-dev/alma/api/v1"
	"github.com/inf0-dev/alma/internal/pkg/engine"
	"github.com/inf0-dev/alma/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertRecentTimestamp(t *testing.T, value string) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err, "DecidedOn should be valid RFC3339")
	assert.WithinDuration(t, time.Now().UTC(), parsed, 5*time.Second, "DecidedOn should be within 5s of now")
}

func baseDocument() *v1.Document {
	return &v1.Document{
		Metadata: v1.Metadata{Version: v1.VersionV1},
		Schema: v1.Schema{
			Title:      "Test",
			Categories: []string{"report", "export"},
			Requirements: []v1.Requirement{
				{ID: "hard1", Description: "Hard req", IsHard: true},
				{ID: "soft1", Description: "Soft req", IsHard: false},
			},
			Items: []v1.Item{
				{
					ID: "q1", Kind: v1.KindChoice, Description: "Q1",
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "yes", Description: "Yes"},
						{ID: "no", Description: "No"},
					},
				},
				{
					ID: "q2", Kind: v1.KindChoice, Description: "Q2",
					ChoiceOptions: []v1.ChoiceOption{
						{ID: "a", Description: "A"},
						{ID: "b", Description: "B"},
					},
				},
			},
			DesignOptions: []v1.DesignOption{
				{
					ID:    "opt1",
					Title: "Option 1",
					RequirementsMet: v1.RequirementsMet{
						"hard1": {Met: true},
						"soft1": {Met: true},
					},
					Effects: map[string][]v1.Effect{
						"q1.yes": {{Kind: v1.EffectChanges, Category: "report", Text: "Rows visible"}},
						"q1.no":  {{Kind: v1.EffectNote, Category: "export", Text: "No export change"}},
					},
				},
				{
					ID:    "opt2",
					Title: "Option 2",
					RequirementsMet: v1.RequirementsMet{
						"hard1": {Met: false},
						"soft1": {Met: true},
					},
				},
				{
					ID:    "opt3",
					Title: "Option 3",
					RequirementsMet: v1.RequirementsMet{
						"hard1": {Met: true},
						"soft1": {Partial: true, Reason: "partially"},
					},
					Blocks: []v1.Block{
						{Condition: []string{"q1.yes", "q2.a"}, Reason: "Combo breaks it"},
					},
				},
			},
		},
	}
}

func baseRequirements() []v1.RecordRequirement {
	return []v1.RecordRequirement{
		{ID: "hard1", IsHard: true, Checked: true},
		{ID: "soft1", IsHard: false, Checked: true},
	}
}

func TestEvaluateReturnsRecord(t *testing.T) {
	doc := baseDocument()
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
	}

	rec, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, v1.RecordVersionV1, rec.Version)
	assert.NotEmpty(t, rec.ModelSHA256)
	assert.Equal(t, *doc, rec.Model)
	assert.Nil(t, rec.Final)
	assert.Empty(t, rec.DecidedOn)
}

func TestEvaluateOptionStatus(t *testing.T) {
	tests := []struct {
		name       string
		items      []v1.RecordItem
		reqs       []v1.RecordRequirement
		wantStatus map[string]v1.OptionStatus
	}{
		{
			name: "all answered, hard req checked — opt2 eliminated, opt3 blocked",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
			},
			reqs: baseRequirements(),
			wantStatus: map[string]v1.OptionStatus{
				"opt1": v1.StatusPossible,
				"opt2": v1.StatusEliminated,
				"opt3": v1.StatusBlocked,
			},
		},
		{
			name: "block does not fire when not all conditions met",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("b")},
			},
			reqs: baseRequirements(),
			wantStatus: map[string]v1.OptionStatus{
				"opt1": v1.StatusPossible,
				"opt2": v1.StatusEliminated,
				"opt3": v1.StatusPossible,
			},
		},
		{
			name: "unchecked hard req does not mute",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
			},
			reqs: []v1.RecordRequirement{
				{ID: "hard1", IsHard: true, Checked: false},
				{ID: "soft1", IsHard: false, Checked: true},
			},
			wantStatus: map[string]v1.OptionStatus{
				"opt1": v1.StatusPossible,
				"opt2": v1.StatusPossible,
				"opt3": v1.StatusPossible,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := engine.Evaluate(baseDocument(), tt.reqs, tt.items)
			require.NoError(t, err)
			require.Len(t, rec.Options, 3)

			for _, opt := range rec.Options {
				expected, ok := tt.wantStatus[opt.ID]
				require.True(t, ok, "unexpected option ID: %s", opt.ID)
				assert.Equal(t, expected, opt.Status, "option %s", opt.ID)
			}
		})
	}
}

func TestEvaluateMeetsCounts(t *testing.T) {
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
	}

	rec, err := engine.Evaluate(baseDocument(), baseRequirements(), items)
	require.NoError(t, err)

	optByID := make(map[string]v1.RecordOption)
	for _, o := range rec.Options {
		optByID[o.ID] = o
	}

	// opt1: meets both hard and soft
	assert.Equal(t, v1.MeetsCounts{Met: 1, Of: 1}, optByID["opt1"].Meets.Hard)
	assert.Equal(t, v1.MeetsCounts{Met: 1, Of: 1}, optByID["opt1"].Meets.Soft)

	// opt2: fails hard, meets soft
	assert.Equal(t, v1.MeetsCounts{Met: 0, Of: 1}, optByID["opt2"].Meets.Hard)
	assert.Equal(t, v1.MeetsCounts{Met: 1, Of: 1}, optByID["opt2"].Meets.Soft)

	// opt3: meets hard, partial soft counts as met
	assert.Equal(t, v1.MeetsCounts{Met: 1, Of: 1}, optByID["opt3"].Meets.Hard)
	assert.Equal(t, v1.MeetsCounts{Met: 1, Of: 1}, optByID["opt3"].Meets.Soft)
}

func TestEvaluateFailedRequirements(t *testing.T) {
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
	}

	rec, err := engine.Evaluate(baseDocument(), baseRequirements(), items)
	require.NoError(t, err)

	optByID := make(map[string]v1.RecordOption)
	for _, o := range rec.Options {
		optByID[o.ID] = o
	}

	assert.Empty(t, optByID["opt1"].FailedRequirements)
	assert.Equal(t, []string{"hard1"}, optByID["opt2"].FailedRequirements)
	assert.Empty(t, optByID["opt3"].FailedRequirements)
}

func TestEvaluateFiredBlocks(t *testing.T) {
	tests := []struct {
		name         string
		items        []v1.RecordItem
		wantFired    []string
		wantNotFired bool
	}{
		{
			name: "block fires when all conditions met",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
			},
			wantFired: []string{"Combo breaks it"},
		},
		{
			name: "block does not fire when partial conditions",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("b")},
			},
			wantNotFired: true,
		},
		{
			name: "block does not fire when no answers",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice},
				{ID: "q2", Kind: v1.KindChoice},
			},
			wantNotFired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := engine.Evaluate(baseDocument(), baseRequirements(), tt.items)
			require.NoError(t, err)

			optByID := make(map[string]v1.RecordOption)
			for _, o := range rec.Options {
				optByID[o.ID] = o
			}

			opt3 := optByID["opt3"]
			if tt.wantNotFired {
				assert.Empty(t, opt3.FiredBlocks)
			} else {
				assert.Equal(t, tt.wantFired, opt3.FiredBlocks)
			}
		})
	}
}

func TestEvaluateFiredEffects(t *testing.T) {
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
	}

	rec, err := engine.Evaluate(baseDocument(), baseRequirements(), items)
	require.NoError(t, err)

	optByID := make(map[string]v1.RecordOption)
	for _, o := range rec.Options {
		optByID[o.ID] = o
	}

	// opt1 has effects for q1.yes — should fire
	opt1Effects := optByID["opt1"].Effects
	require.Len(t, opt1Effects, 1)
	assert.Equal(t, "q1", opt1Effects[0].Item)
	assert.Equal(t, "yes", opt1Effects[0].Answer)
	assert.Equal(t, v1.EffectChanges, opt1Effects[0].Kind)
	assert.Equal(t, "report", opt1Effects[0].Category)
	assert.Equal(t, "Rows visible", opt1Effects[0].Text)

	// opt2 has no effects defined
	assert.Empty(t, optByID["opt2"].Effects)
}

func TestEvaluateEffectsNotFiredForUnchosenAnswer(t *testing.T) {
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
	}

	rec, err := engine.Evaluate(baseDocument(), baseRequirements(), items)
	require.NoError(t, err)

	optByID := make(map[string]v1.RecordOption)
	for _, o := range rec.Options {
		optByID[o.ID] = o
	}

	// q1.yes effects should NOT fire, q1.no effects should fire
	opt1Effects := optByID["opt1"].Effects
	require.Len(t, opt1Effects, 1)
	assert.Equal(t, "no", opt1Effects[0].Answer)
	assert.Equal(t, v1.EffectNote, opt1Effects[0].Kind)
}

func TestEvaluateStillOpen(t *testing.T) {
	tests := []struct {
		name     string
		items    []v1.RecordItem
		wantOpen []string
	}{
		{
			name: "all answered",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
			},
			wantOpen: nil,
		},
		{
			name: "one unanswered",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
				{ID: "q2", Kind: v1.KindChoice},
			},
			wantOpen: []string{"q2"},
		},
		{
			name: "all unanswered",
			items: []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice},
				{ID: "q2", Kind: v1.KindChoice},
			},
			wantOpen: []string{"q1", "q2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := engine.Evaluate(baseDocument(), baseRequirements(), tt.items)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOpen, rec.StillOpen)
		})
	}
}

// --- ShowWhen tests ---

func docWithShowWhen() *v1.Document {
	doc := baseDocument()
	doc.Schema.Items = append(doc.Schema.Items, v1.Item{
		ID: "q3", Kind: v1.KindChoice, Description: "Q3 (depends on q1=yes)",
		ChoiceOptions: []v1.ChoiceOption{
			{ID: "x", Description: "X"},
			{ID: "y", Description: "Y"},
		},
		ShowWhen: [][]string{{"q1.yes"}},
	})
	// Add q3 requirements_met to all options
	for i := range doc.Schema.DesignOptions {
		if doc.Schema.DesignOptions[i].RequirementsMet == nil {
			doc.Schema.DesignOptions[i].RequirementsMet = v1.RequirementsMet{}
		}
	}
	return doc
}

func TestEvaluateShowWhenHidesItem(t *testing.T) {
	doc := docWithShowWhen()
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
		{ID: "q3", Kind: v1.KindChoice, Answer: testutil.StrPtr("x")},
	}

	rec, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)

	// q3 should be hidden (q1=no, not q1=yes)
	for _, item := range rec.Items {
		if item.ID == "q3" {
			require.NotNil(t, item.Visible)
			assert.False(t, *item.Visible, "q3 should be hidden when q1=no")
		}
	}
}

func TestEvaluateShowWhenShowsItem(t *testing.T) {
	doc := docWithShowWhen()
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
		{ID: "q3", Kind: v1.KindChoice, Answer: testutil.StrPtr("x")},
	}

	rec, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)

	for _, item := range rec.Items {
		if item.ID == "q3" {
			require.NotNil(t, item.Visible)
			assert.True(t, *item.Visible, "q3 should be visible when q1=yes")
		}
	}
}

func TestEvaluateShowWhenHiddenItemNotInActiveKeys(t *testing.T) {
	doc := docWithShowWhen()
	// Add a block that depends on q3.x
	doc.Schema.DesignOptions[0].Blocks = []v1.Block{
		{Condition: []string{"q3.x"}, Reason: "q3 blocks it"},
	}

	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
		{ID: "q3", Kind: v1.KindChoice, Answer: testutil.StrPtr("x")},
	}

	rec, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)

	optByID := make(map[string]v1.RecordOption)
	for _, o := range rec.Options {
		optByID[o.ID] = o
	}

	// q3 is hidden (q1=no), so q3.x should NOT be in active keys, block should NOT fire
	assert.Empty(t, optByID["opt1"].FiredBlocks, "block should not fire when dependent item is hidden")
}

func TestEvaluateShowWhenHiddenNotInStillOpen(t *testing.T) {
	doc := docWithShowWhen()
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
		{ID: "q3", Kind: v1.KindChoice}, // unanswered but hidden
	}

	rec, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)
	assert.NotContains(t, rec.StillOpen, "q3", "hidden unanswered items should not be in still_open")
}

func TestEvaluateShowWhenCascade(t *testing.T) {
	doc := baseDocument()
	// q3 depends on q1=yes, q4 depends on q3=x
	doc.Schema.Items = append(doc.Schema.Items,
		v1.Item{
			ID: "q3", Kind: v1.KindChoice, Description: "Q3",
			ChoiceOptions: []v1.ChoiceOption{{ID: "x", Description: "X"}},
			ShowWhen:      [][]string{{"q1.yes"}},
		},
		v1.Item{
			ID: "q4", Kind: v1.KindChoice, Description: "Q4",
			ChoiceOptions: []v1.ChoiceOption{{ID: "z", Description: "Z"}},
			ShowWhen:      [][]string{{"q3.x"}},
		},
	)

	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("no")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
		{ID: "q3", Kind: v1.KindChoice, Answer: testutil.StrPtr("x")},
		{ID: "q4", Kind: v1.KindChoice, Answer: testutil.StrPtr("z")},
	}

	rec, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)

	for _, item := range rec.Items {
		if item.ID == "q3" || item.ID == "q4" {
			require.NotNil(t, item.Visible)
			assert.False(t, *item.Visible, "%s should be hidden (cascade)", item.ID)
		}
	}
}

func TestEvaluateShowWhenORGroups(t *testing.T) {
	doc := baseDocument()
	// q3 visible if q1=yes OR q2=b
	doc.Schema.Items = append(doc.Schema.Items, v1.Item{
		ID: "q3", Kind: v1.KindChoice, Description: "Q3",
		ChoiceOptions: []v1.ChoiceOption{{ID: "x", Description: "X"}},
		ShowWhen:      [][]string{{"q1.yes"}, {"q2.b"}},
	})

	tests := []struct {
		name    string
		q1      string
		q2      string
		visible bool
	}{
		{"q1=yes satisfies first group", "yes", "a", true},
		{"q2=b satisfies second group", "no", "b", true},
		{"neither satisfied", "no", "a", false},
		{"both satisfied", "yes", "b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := []v1.RecordItem{
				{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr(tt.q1)},
				{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr(tt.q2)},
				{ID: "q3", Kind: v1.KindChoice},
			}
			rec, err := engine.Evaluate(doc, baseRequirements(), items)
			require.NoError(t, err)

			for _, item := range rec.Items {
				if item.ID == "q3" {
					require.NotNil(t, item.Visible)
					assert.Equal(t, tt.visible, *item.Visible)
				}
			}
		})
	}
}

// --- Export tests ---

func TestExportNoPrevious(t *testing.T) {
	curr := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "hash1",
		DecidedOn:   "2026-10-01",
		Present:     []string{"Alice"},
		Final:       &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Best"},
	}

	out := engine.Export(nil, curr)
	require.NotNil(t, out)
	assert.Empty(t, out.History)
	assert.Equal(t, curr.Final, out.Final)
	assertRecentTimestamp(t, out.DecidedOn)
}

func TestExportWithPrevious(t *testing.T) {
	prev := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "old_hash",
		DecidedOn:   "2026-09-01",
		Present:     []string{"Bob"},
		Final:       &v1.FinalDecision{Option: "opt2", Title: "Option 2", Rationale: "Was best then"},
	}

	curr := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "new_hash",
		DecidedOn:   "2026-10-01",
		Present:     []string{"Alice"},
		Final:       &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Better now"},
	}

	out := engine.Export(prev, curr)
	require.NotNil(t, out)

	expectedHistory := []v1.HistoryEntry{
		{
			DecidedOn:   "2026-09-01",
			Present:     []string{"Bob"},
			Option:      "opt2",
			Title:       "Option 2",
			Rationale:   "Was best then",
			ModelSHA256: "old_hash",
		},
	}
	assert.Equal(t, expectedHistory, out.History)
	assert.Equal(t, curr.Final, out.Final)
	assertRecentTimestamp(t, out.DecidedOn)
}

func TestExportPreservesExistingHistory(t *testing.T) {
	prev := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "hash2",
		DecidedOn:   "2026-09-15",
		Present:     []string{"Charlie"},
		Final:       &v1.FinalDecision{Option: "opt2", Title: "Option 2", Rationale: "Second pick"},
		History: []v1.HistoryEntry{
			{
				DecidedOn:   "2026-08-01",
				Present:     []string{"Dave"},
				Option:      "opt3",
				Title:       "Option 3",
				Rationale:   "First pick",
				ModelSHA256: "hash1",
			},
		},
	}

	curr := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "hash3",
		DecidedOn:   "2026-10-01",
		Final:       &v1.FinalDecision{Option: "opt1", Title: "Option 1", Rationale: "Third pick"},
	}

	out := engine.Export(prev, curr)

	expectedHistory := []v1.HistoryEntry{
		{
			DecidedOn:   "2026-08-01",
			Present:     []string{"Dave"},
			Option:      "opt3",
			Title:       "Option 3",
			Rationale:   "First pick",
			ModelSHA256: "hash1",
		},
		{
			DecidedOn:   "2026-09-15",
			Present:     []string{"Charlie"},
			Option:      "opt2",
			Title:       "Option 2",
			Rationale:   "Second pick",
			ModelSHA256: "hash2",
		},
	}
	assert.Equal(t, expectedHistory, out.History)
}

func TestExportPreviousWithNoFinal(t *testing.T) {
	prev := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "old_hash",
		DecidedOn:   "2026-09-01",
		Final:       nil, // no decision was made
	}

	curr := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "new_hash",
		DecidedOn:   "2026-10-01",
		Final:       &v1.FinalDecision{Option: "opt1", Title: "Option 1"},
	}

	out := engine.Export(prev, curr)
	assert.Empty(t, out.History) // nothing to move
}

func TestExportDoesNotMutatePrevOrCurr(t *testing.T) {
	prev := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "old_hash",
		DecidedOn:   "2026-09-01",
		Final:       &v1.FinalDecision{Option: "opt2", Title: "Option 2"},
		History:     []v1.HistoryEntry{},
	}

	curr := &v1.Record{
		Version:     v1.RecordVersionV1,
		ModelSHA256: "new_hash",
		DecidedOn:   "2026-10-01",
		Final:       &v1.FinalDecision{Option: "opt1", Title: "Option 1"},
	}

	_ = engine.Export(prev, curr)

	// prev and curr should be unchanged
	assert.Empty(t, prev.History)
	assert.Nil(t, curr.History)
}

func TestEvaluateModelSHA256Consistent(t *testing.T) {
	doc := baseDocument()
	items := []v1.RecordItem{
		{ID: "q1", Kind: v1.KindChoice, Answer: testutil.StrPtr("yes")},
		{ID: "q2", Kind: v1.KindChoice, Answer: testutil.StrPtr("a")},
	}

	rec1, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)

	rec2, err := engine.Evaluate(doc, baseRequirements(), items)
	require.NoError(t, err)

	assert.Equal(t, rec1.ModelSHA256, rec2.ModelSHA256)
	assert.NotEmpty(t, rec1.ModelSHA256)
}
