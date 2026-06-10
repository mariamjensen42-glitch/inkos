package model

// PlayActionKind is a verb the user can take inside a Play run.
type PlayActionKind string

const (
	PlayActionFree         PlayActionKind = "free"
	PlayActionChoice       PlayActionKind = "choice"
	PlayActionInspect      PlayActionKind = "inspect"
	PlayActionUse          PlayActionKind = "use"
	PlayActionTalk         PlayActionKind = "talk"
	PlayActionMove         PlayActionKind = "move"
	PlayActionWait         PlayActionKind = "wait"
)

// PlayEntityType is a class of object in the world.
type PlayEntityType string

const (
	PlayEntityCharacter  PlayEntityType = "character"
	PlayEntityItem       PlayEntityType = "item"
	PlayEntityEvidence   PlayEntityType = "evidence"
	PlayEntityLocation   PlayEntityType = "location"
)

// PlayVisibility controls who can see an entity.
type PlayVisibility string

const (
	PlayVisibilityPublic   PlayVisibility = "public"
	PlayVisibilityPrivate  PlayVisibility = "private"
	PlayVisibilityHidden   PlayVisibility = "hidden"
)

// PlayEntity is a node in the world graph.
type PlayEntity struct {
	ID          string          `json:"id"`
	Type        PlayEntityType  `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Visibility  PlayVisibility  `json:"visibility,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Rarity      string          `json:"rarity,omitempty"`
	ImagePrompt string          `json:"imagePrompt,omitempty"`
	ImageFile   string          `json:"imageFile,omitempty"`
}

// PlayEdge is a relation between two entities.
type PlayEdge struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Label  string `json:"label,omitempty"`
	Weight int    `json:"weight,omitempty"`
}

// PlayStateSlotKind classifies a stateful attribute of an entity.
type PlayStateSlotKind string

const (
	PlayStateSlotNumeric  PlayStateSlotKind = "numeric"
	PlayStateSlotText     PlayStateSlotKind = "text"
	PlayStateSlotBoolean  PlayStateSlotKind = "boolean"
	PlayStateSlotEnum     PlayStateSlotKind = "enum"
)

// PlayStateSlot is a typed state attribute of a single entity.
type PlayStateSlot struct {
	ID        string            `json:"id"`
	EntityID  string            `json:"entityId"`
	Kind      PlayStateSlotKind `json:"kind"`
	Key       string            `json:"key"`
	ValueNum  *float64          `json:"valueNum,omitempty"`
	ValueText *string           `json:"valueText,omitempty"`
	ValueBool *bool             `json:"valueBool,omitempty"`
	ValueEnum *string           `json:"valueEnum,omitempty"`
}

// PlayEvidenceStatus is the lifecycle of an evidence object.
type PlayEvidenceStatus string

const (
	PlayEvidenceUncollected PlayEvidenceStatus = "uncollected"
	PlayEvidenceCollected   PlayEvidenceStatus = "collected"
	PlayEvidenceAnalyzed    PlayEvidenceStatus = "analyzed"
	PlayEvidenceSpent       PlayEvidenceStatus = "spent"
)

// PlayEvent is an immutable narrative event in a run.
type PlayEvent struct {
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	AtChapter int                    `json:"atChapter"`
	Text      string                 `json:"text"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	CreatedAt string                 `json:"createdAt"`
}

// PlayMutation is a pending state change to be applied by the reducer.
type PlayMutation struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	Payload    map[string]interface{} `json:"payload"`
	CreatedAt  string                 `json:"createdAt"`
}

// PlayRunSummary describes a single run.
type PlayRunSummary struct {
	WorldID    string `json:"worldId"`
	RunID      string `json:"runId"`
	Title      string `json:"title"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Turn       int    `json:"turn"`
	Chapter    int    `json:"chapter"`
}
