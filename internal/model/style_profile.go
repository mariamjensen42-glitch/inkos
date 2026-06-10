package model

// StyleProfile captures a writing-style fingerprint extracted from a sample.
type StyleProfile struct {
	ID                string             `json:"id"`
	SourceFile        string             `json:"sourceFile,omitempty"`
	Language          string             `json:"language"`
	SentenceLength    SentenceLengthBuckets `json:"sentenceLength"`
	TopTokens         []TokenCount        `json:"topTokens"`
	TopBigrams        []TokenCount        `json:"topBigrams"`
	Tells             []string            `json:"tells,omitempty"`
	StyleGuide        string             `json:"styleGuide,omitempty"`
	CreatedAt         string             `json:"createdAt"`
}

// SentenceLengthBuckets is a histogram of sentence lengths in words.
type SentenceLengthBuckets struct {
	Short  int `json:"short"`
	Medium int `json:"medium"`
	Long   int `json:"long"`
	VeryLong int `json:"veryLong"`
}

// TokenCount is a single token with its frequency.
type TokenCount struct {
	Token string `json:"token"`
	Count int    `json:"count"`
}

// DetectionHistoryEntry is one recorded AIGC detection run.
type DetectionHistoryEntry struct {
	Chapter   int       `json:"chapter"`
	Timestamp string    `json:"timestamp"`
	Score     float64   `json:"score"`
	Verdict   string    `json:"verdict"`
	Notes     string    `json:"notes,omitempty"`
	PerDim    map[string]float64 `json:"perDim,omitempty"`
}

// DetectionStats is aggregate detection telemetry.
type DetectionStats struct {
	TotalChecked  int                       `json:"totalChecked"`
	AvgScore      float64                   `json:"avgScore"`
	HighRiskCount int                       `json:"highRiskCount"`
	PerChapter    []DetectionHistoryEntry   `json:"perChapter"`
}
