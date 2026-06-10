package model

// PendingHookStatus is the lifecycle of a story hook (foreshadow).
type PendingHookStatus string

const (
	HookStatusOpen        PendingHookStatus = "open"
	HookStatusProgressing PendingHookStatus = "progressing"
	HookStatusDeferred    PendingHookStatus = "deferred"
	HookStatusResolved    PendingHookStatus = "resolved"
)

// PendingHook is a single hook entry in pending_hooks.md / story/state.
type PendingHook struct {
	ID                  string            `json:"id"`
	Description         string            `json:"description"`
	Status              PendingHookStatus `json:"status"`
	LastAdvancedChapter int               `json:"lastAdvancedChapter"`
	CreatedAtChapter    int               `json:"createdAtChapter"`
	RelatedCharacters   []string          `json:"relatedCharacters,omitempty"`
	Importance          int               `json:"importance,omitempty"`
	Tags                []string          `json:"tags,omitempty"`
}

// PendingHooks is the list of active hooks.
type PendingHooks struct {
	Hooks []PendingHook `json:"hooks"`
}

// ChapterSummaryRow is a row in chapter_summaries.md.
type ChapterSummaryRow struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ChapterSummariesState is the structured chapter summary state.
type ChapterSummariesState struct {
	Summaries []ChapterSummaryRow `json:"summaries"`
}

// StateManifest describes what state files exist for a book.
type StateManifest struct {
	Version     int      `json:"version"`
	HasLegacy   bool     `json:"hasLegacy"`
	TruthFiles  []string `json:"truthFiles"`
	StateFiles  []string `json:"stateFiles"`
	MemoryDB    bool     `json:"memoryDb"`
	PlayDB      bool     `json:"playDb"`
}

// LengthSpec governs chapter length expectations.
type LengthSpec struct {
	Mode       LengthCountingMode `json:"mode"`
	Target     int                `json:"target"`
	Min        int                `json:"min"`
	Max        int                `json:"max"`
	Tolerance  int                `json:"tolerance,omitempty"`
	HardRange  []int              `json:"hardRange,omitempty"`
}

// LengthCountingMode decides how length is measured.
type LengthCountingMode string

const (
	LengthModeZhChars LengthCountingMode = "zh_chars"
	LengthModeEnWords LengthCountingMode = "en_words"
)
