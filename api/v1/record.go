package v1

// RecordVersionV1 is the version string for the v1 record schema.
const RecordVersionV1 = "alma/record/v1"

// OptionStatus represents the derived status of a design option in the record.
type OptionStatus string

const (
	// StatusPicked means this option was selected as the final decision.
	StatusPicked OptionStatus = "picked"
	// StatusPossible means this option is viable but was not picked.
	StatusPossible OptionStatus = "possible"
	// StatusEliminated means this option fails a checked hard requirement.
	StatusEliminated OptionStatus = "eliminated"
	// StatusBlocked means an answer combination makes this option unworkable.
	StatusBlocked OptionStatus = "blocked"
)

// MeetsCounts holds the count of met vs total requirements for a category (hard or soft).
type MeetsCounts struct {
	Met int `json:"met" yaml:"met"`
	Of  int `json:"of" yaml:"of"`
}

// RecordRequirement captures a requirement's checked state in the record.
type RecordRequirement struct {
	// ID is the requirement ID from the input model.
	ID string `json:"id" yaml:"id"`
	// IsHard indicates whether the requirement is hard.
	IsHard bool `json:"is_hard" yaml:"is_hard"`
	// Checked indicates whether this requirement was checked/active during the session.
	Checked bool `json:"checked" yaml:"checked"`
}

// RecordItem captures an item's answer or value in the record.
type RecordItem struct {
	// ID is the item ID from the input model.
	ID string `json:"id" yaml:"id"`
	// Kind is the item kind from the input model.
	Kind Kind `json:"kind" yaml:"kind"`
	// Answer is the selected choice option ID. Only for choice items. Nil if unanswered.
	Answer *string `json:"answer,omitempty" yaml:"answer,omitempty"`
	// Value is the free-form value for text, number, or people items.
	Value any `json:"value,omitempty" yaml:"value,omitempty"`
	// Unit is the unit for number items, carried from the input model for readability.
	Unit string `json:"unit,omitempty" yaml:"unit,omitempty"`
	// Visible indicates whether the item is visible given current show_when conditions. Derived field.
	Visible *bool `json:"visible,omitempty" yaml:"visible,omitempty"`
}

// FiredEffect is an effect that fired for the chosen answers.
type FiredEffect struct {
	// Item is the item ID the effect is keyed to.
	Item string `json:"item" yaml:"item"`
	// Answer is the answer ID the effect is keyed to.
	Answer string `json:"answer" yaml:"answer"`
	// Kind is the effect kind (changes or note).
	Kind EffectKind `json:"kind" yaml:"kind"`
	// Category is the effect category.
	Category string `json:"category,omitempty" yaml:"category,omitempty"`
	// Text is the effect description.
	Text string `json:"text" yaml:"text"`
}

// RecordOption holds the derived status and details for a design option in the record.
// These fields are recomputed on import and never trusted from the file.
type RecordOption struct {
	// ID is the design option ID.
	ID string `json:"id" yaml:"id"`
	// Status is the derived status: picked, possible, eliminated, or blocked.
	Status OptionStatus `json:"status" yaml:"status"`
	// Meets holds the count of met requirements by category.
	Meets struct {
		Hard MeetsCounts `json:"hard" yaml:"hard"`
		Soft MeetsCounts `json:"soft" yaml:"soft"`
	} `json:"meets" yaml:"meets"`
	// FailedRequirements lists requirement IDs that this option fails.
	FailedRequirements []string `json:"failed_requirements" yaml:"failed_requirements"`
	// FiredBlocks lists the reasons from blocks that fired.
	FiredBlocks []string `json:"fired_blocks" yaml:"fired_blocks"`
	// Effects lists only the effects for the currently chosen answers.
	Effects []FiredEffect `json:"effects,omitempty" yaml:"effects,omitempty"`
}

// FinalDecision captures the picked option and rationale.
type FinalDecision struct {
	// Option is the ID of the picked design option.
	Option string `json:"option" yaml:"option"`
	// Title is the title of the picked option, for readability.
	Title string `json:"title" yaml:"title"`
	// Rationale explains why this option was picked.
	Rationale string `json:"rationale,omitempty" yaml:"rationale,omitempty"`
}

// HistoryEntry represents a previous decision, preserved when a record is re-exported.
type HistoryEntry struct {
	// DecidedOn is the date of the previous decision.
	DecidedOn string `json:"decided_on" yaml:"decided_on"`
	// Present lists who was at the previous session.
	Present []string `json:"present,omitempty" yaml:"present,omitempty"`
	// Option is the ID of the previously picked design option.
	Option string `json:"option" yaml:"option"`
	// Title is the title of the previously picked option.
	Title string `json:"title" yaml:"title"`
	// Rationale explains why the previous option was picked.
	Rationale string `json:"rationale,omitempty" yaml:"rationale,omitempty"`
	// ModelSHA256 is the hash of the model at the time of the previous decision.
	ModelSHA256 string `json:"model_sha256" yaml:"model_sha256"`
}

// Record is the decision record: the output of an alma session.
// It embeds the full input model so one file is enough to reopen a decision.
type Record struct {
	// Version is the record schema version.
	Version string `json:"version" yaml:"version"`
	// Model is the full input document, embedded verbatim.
	Model Document `json:"model" yaml:"model"`
	// ModelSHA256 is the SHA-256 hash of the canonical JSON of Model. Used to detect model changes across history.
	ModelSHA256 string `json:"model_sha256" yaml:"model_sha256"`
	// DecidedOn is the date of this decision.
	DecidedOn string `json:"decided_on" yaml:"decided_on"`
	// Present lists who was at this session.
	Present []string `json:"present,omitempty" yaml:"present,omitempty"`
	// Requirements captures the checked state of each requirement.
	Requirements []RecordRequirement `json:"requirements" yaml:"requirements"`
	// Items captures the answers and values for each item.
	Items []RecordItem `json:"items" yaml:"items"`
	// Options holds derived status per design option. Recomputed on import.
	Options []RecordOption `json:"options" yaml:"options"`
	// Final is the picked option and rationale. Nil if no decision was made yet.
	Final *FinalDecision `json:"final,omitempty" yaml:"final,omitempty"`
	// StillOpen lists item IDs that have no answer yet.
	StillOpen []string `json:"still_open,omitempty" yaml:"still_open,omitempty"`
	// History holds previous decisions, oldest first. Appended on re-export.
	History []HistoryEntry `json:"history,omitempty" yaml:"history,omitempty"`
}
