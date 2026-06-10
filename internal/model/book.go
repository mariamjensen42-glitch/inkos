// Package model defines all data structures used by the InkOS API.
// These mirror the TypeScript models from the original InkOS core package,
// translated to idiomatic Go with snake_case JSON tags to keep file
// compatibility with existing project data.
package model

// Platform identifies a publishing platform (e.g. qidian, fanqie, other).
type Platform string

const (
	PlatformQidian Platform = "qidian"
	PlatformFanqie Platform = "fanqie"
	PlatformOther  Platform = "other"
)

// Genre identifies a story genre. Values are loaded from
// packages/core/genres/<id>.md frontmatter.
type Genre string

// BookStatus is the lifecycle status of a book.
type BookStatus string

const (
	BookStatusActive   BookStatus = "active"
	BookStatusPaused   BookStatus = "paused"
	BookStatusFinished BookStatus = "finished"
	BookStatusArchived BookStatus = "archived"
)

// FanficMode controls how a fanfic book relates to its source material.
type FanficMode string

const (
	FanficModeCanon FanficMode = "canon"
	FanficModeAU    FanficMode = "au"
	FanficModeOOC   FanficMode = "ooc"
	FanficModeCP    FanficMode = "cp"
)

// BookConfig is the persistent configuration for a single book.
type BookConfig struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Genre           Genre     `json:"genre"`
	Platform        Platform  `json:"platform"`
	ChapterWordCount int      `json:"chapterWordCount"`
	TargetChapters  int       `json:"targetChapters"`
	Status          BookStatus `json:"status"`
	Language        string    `json:"language,omitempty"`
	Blurb           string    `json:"blurb,omitempty"`
	Brief           string    `json:"brief,omitempty"`
	FanficMode      FanficMode `json:"fanficMode,omitempty"`
	SourceBookID    string    `json:"sourceBookId,omitempty"`
	CreatedAt       string    `json:"createdAt"`
	UpdatedAt       string    `json:"updatedAt"`
}

// ChapterStatus is the lifecycle status of a chapter.
type ChapterStatus string

const (
	ChapterStatusDraft     ChapterStatus = "draft"
	ChapterStatusAuditing  ChapterStatus = "auditing"
	ChapterStatusRevising  ChapterStatus = "revising"
	ChapterStatusApproved  ChapterStatus = "approved"
	ChapterStatusRejected  ChapterStatus = "rejected"
	ChapterStatusPending   ChapterStatus = "pending"
)

// ChapterMeta is a row in the chapter index.
type ChapterMeta struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Status      ChapterStatus `json:"status"`
	WordCount   int           `json:"wordCount"`
	CharCount   int           `json:"charCount,omitempty"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt"`
	ApprovedAt  *string       `json:"approvedAt,omitempty"`
	WordWarning *string       `json:"wordWarning,omitempty"`
}

// ChapterIndex is the chapter index file persisted at books/<id>/chapters/index.json.
type ChapterIndex struct {
	Chapters []ChapterMeta `json:"chapters"`
}
