package v1

// VersionV1 is the version string for the v1 schema.
const VersionV1 = "alma/v1"

// Requirement represents a single requirement in the alma.
type Requirement struct {
	// ID is the unique identifier as a string for the requirement.
	ID string `json:"id" yaml:"id"`
	// Description is a human-readable description of the requirement.
	Description string `json:"description" yaml:"description"`
	// IsHard indicates whether the requirement is a hard requirement. If it is, it mutes design options that fail it, if it's not (i.e. "soft requirement"), it is only used for tracking.
	IsHard bool `json:"is_hard" yaml:"is_hard"`
}

type Kind string

const (
	// KindChoice represents a choice item in the alma. These have a direct effect on design options available.
	KindChoice Kind = "choice"
	// KindText represents a text item in the alma. These are used for tracking and do not affect design options (no easy way to define "S text means option X is not possible").
	KindText Kind = "text"
	// KindNumber represents a number item in the alma. These are used for tracking and do not affect design options (no easy way to define "S number means option X is not possible").
	// In the future, there may be a way do define conditions for number items (e.g. "if {number} > {value}".)
	KindNumber Kind = "number"
)

// ChoiceOption represents a single option for a Choice item in the alma.
type ChoiceOption struct {
	// ID is the unique identifier as a string for the choice option.
	ID string `json:"id" yaml:"id"`
	// Description is a human-readable description of the choice option.
	Description string `json:"description" yaml:"description"`
}

// NumberConfig holds configuration specific to number items.
type NumberConfig struct {
	// Unit is the unit of measurement (e.g. "months", "days").
	Unit string `json:"unit,omitempty" yaml:"unit,omitempty"`
	// Min is the optional minimum value.
	Min *int `json:"min,omitempty" yaml:"min,omitempty"`
	// Max is the optional maximum value.
	Max *int `json:"max,omitempty" yaml:"max,omitempty"`
}

// Item represents a single item in the alma.
type Item struct {
	// ID is the unique identifier as a string for the item.
	ID string `json:"id" yaml:"id"`
	// Description is a human-readable description of the item.
	Description string `json:"description" yaml:"description"`
	// Kind indicates the kind of item (Choice, Text, or Number).
	Kind Kind `json:"kind" yaml:"kind"`
	// ChoiceOptions is a list of options for the item. This is only applicable for Choice items.
	ChoiceOptions []ChoiceOption `json:"choice_options,omitempty" yaml:"choice_options,omitempty"`
	// Note is an optional clarification about the item.
	Note string `json:"note,omitempty" yaml:"note,omitempty"`
	// NumberConfig holds configuration specific to number items. Only valid when Kind is KindNumber.
	NumberConfig *NumberConfig `json:"number_config,omitempty" yaml:"number_config,omitempty"`
}

// RequirementStatus represents whether a design option meets a requirement: fully, not at all, or partially with a reason.
type RequirementStatus struct {
	// Met indicates whether the requirement is met.
	Met bool `json:"met" yaml:"met"`
	// Partial indicates whether the requirement is partially met. When true, Reason should explain why.
	Partial bool `json:"partial,omitempty" yaml:"partial,omitempty"`
	// Reason explains why a requirement is only partially met.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// RequirementsMet is a map that indicates which requirements are met by a design option. The keys are requirement IDs.
type RequirementsMet map[string]RequirementStatus

// EffectKind indicates the type of effect.
type EffectKind string

const (
	// EffectChanges means the option behaves differently under this answer.
	EffectChanges EffectKind = "changes"
	// EffectNote means the effect is worth knowing but doesn't change whether the option works.
	EffectNote EffectKind = "note"
)

// Effect describes what changes about a design option when a particular choice answer is selected.
type Effect struct {
	// Kind is the type of effect: "changes" or "note".
	Kind EffectKind `json:"kind" yaml:"kind"`
	// Category this effect belongs to (e.g. "report", "export"). Must be in Schema.Categories if that list is set.
	Category string `json:"category,omitempty" yaml:"category,omitempty"`
	// Text is an authored sentence describing the effect.
	Text string `json:"text" yaml:"text"`
}

// Block represents an answer combination that makes a design option unworkable.
// All entries in When must be true for the block to fire (AND). Multiple blocks on an option act as OR.
type Block struct {
	// Condition is a list of "<item_id>.<answer_id>" entries that must all be selected for this block to fire.
	Condition []string `json:"condition" yaml:"condition"`
	// Reason explains why this combination makes the option unworkable.
	Reason string `json:"reason" yaml:"reason"`
}

// DesignOption represents a single design option that the alma will evaluate against requirements, items, and blocks.
type DesignOption struct {
	// ID is the unique identifier as a string for the design option.
	ID string `json:"id" yaml:"id"`
	// Title is the title of the design option.
	Title string `json:"title" yaml:"title"`
	// Description is a human-readable description of the design option.
	Description string `json:"description" yaml:"description"`
	// RequirementsMet is a list of requirement IDs that this design option meets.
	RequirementsMet RequirementsMet `json:"requirements_met,omitempty" yaml:"requirements_met,omitempty"`
	// Effects maps "<item_id>.<answer_id>" to a list of effects describing what changes about this option under that answer.
	Effects map[string][]Effect `json:"effects,omitempty" yaml:"effects,omitempty"`
	// Blocks is a list of answer combinations that make this option unworkable. AND within a block, OR across blocks.
	Blocks []Block `json:"blocks,omitempty" yaml:"blocks,omitempty"`
	// Pros is the list of pros for the design option. For display purposes only.
	Pros []string `json:"pros,omitempty" yaml:"pros,omitempty"`
	// Cons is the list of cons for the design option. For display purposes only.
	Cons []string `json:"cons,omitempty" yaml:"cons,omitempty"`
}

// Metadata holds document-level information about the alma session.
type Metadata struct {
	// Version is the schema version. Must match a known version constant (e.g. VersionV1) to select the correct parser/validator.
	Version string `json:"version" yaml:"version"`
	// Author is the person or team who authored this alma document.
	Author string `json:"author,omitempty" yaml:"author,omitempty"`
	// Date is the date of the alignment session.
	Date string `json:"date,omitempty" yaml:"date,omitempty"`
}

// Schema represents the schema for the alma API's inputs.
type Schema struct {
	// Title is the title of the alma (e.g., "alma for Project X").
	Title string `json:"title" yaml:"title"`
	// Issue is the Git issue link associated with the alignment, used for extra display purposes, only the validity of the URL is validated (if set).
	Issue string `json:"issue,omitempty" yaml:"issue,omitempty"`
	// Categories is an optional list of valid category names. When set, every DesignOption Effect.Category must be in this list.
	Categories []string `json:"categories,omitempty" yaml:"categories,omitempty"`
	// Requirements is a list of requirements that the alma will evaluate.
	Requirements []Requirement `json:"requirements,omitempty" yaml:"requirements,omitempty"`
	// Items is a list of items that the alma will evaluate.
	Items []Item `json:"items,omitempty" yaml:"items,omitempty"`
	// DesignOptions is a list of design options that the alma will evaluate.
	DesignOptions []DesignOption `json:"design_options,omitempty" yaml:"design_options,omitempty"`
}

// Document is the top-level container for an alma, combining metadata with the schema definition.
type Document struct {
	// Metadata holds document-level information (version, attendees, date).
	Metadata Metadata `json:"metadata" yaml:"metadata"`
	// Schema defines the alma structure (requirements, items, design options).
	Schema Schema `json:"schema" yaml:"schema"`
}
