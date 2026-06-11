// Package pipeline orchestrates the LLM-driven chapter writing pipeline.
//
// Stages: plan -> compose -> draft -> audit -> (revise -> audit)*
//
// Each stage invokes the corresponding agent under
// internal/agents, persists its output to disk through
// internal/store, and publishes SSE progress events through the
// configured Broadcaster. The Runner is safe for concurrent use across
// different books, but the same book should run through one pipeline
// at a time (the BookStore has a per-book file lock).
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/narcooo/inkos/internal/agents"
	"github.com/narcooo/inkos/internal/llm"
	"github.com/narcooo/inkos/internal/model"
	"github.com/narcooo/inkos/internal/store"
)

// SSEEvent is kept here for backwards compatibility but new code should
// use the Broadcaster.Publish(name, data) shape below.
type SSEEvent struct {
	Event string
	Data  interface{}
}

// Broadcaster is the subset of the api.Broadcaster the runner depends
// on. Defined as an interface here to avoid an import cycle (api ->
// pipeline). The api package's Broadcaster implements this interface
// via its PublishEvent method.
type Broadcaster interface {
	PublishEvent(name string, data interface{})
}

// ResolverFactory returns a fresh resolver. The runner calls it on every
// stage entry so that mid-pipeline config changes are honored.
type ResolverFactory func() *llm.Resolver

// Errors.
var (
	ErrNoResolver = errors.New("no LLM resolver configured")
	ErrNoClient   = errors.New("LLM resolver returned no client")
)

// Runner is the chapter pipeline orchestrator.
type Runner struct {
	Books           *store.BookStore
	Truth           *store.TruthStore
	Project         *store.ProjectStore
	Broadcaster     Broadcaster
	ResolverFactory ResolverFactory
	// MaxChapterContextChars caps how much of each foundation/state file
	// is concatenated into the planner / writer user prompt. 0 = 4000.
	MaxChapterContextChars int
}

// NewRunner constructs a Runner wired to the supplied stores.
func NewRunner(books *store.BookStore, truth *store.TruthStore, project *store.ProjectStore, bc Broadcaster, rf ResolverFactory) *Runner {
	return &Runner{
		Books:                  books,
		Truth:                  truth,
		Project:                project,
		Broadcaster:            bc,
		ResolverFactory:        rf,
		MaxChapterContextChars: 4000,
	}
}

// Options control a single pipeline invocation. All fields are optional.
type Options struct {
	Service    string // LLM service name (empty = project default)
	Words      int    // target word count (0 = inherit from book config)
	Context    string // optional user-provided extra context
	Extra      string // extra free-form text
	Audit      bool   // run audit after draft
	Revise     bool   // auto-revise on audit fail
	MaxRetries int    // override writing.reviewRetries (0 = use project config)
}

// Result is the per-stage output. Body carries the generated artifact
// text (intent / context / chapter / audit JSON). Extras is an
// open-ended map of metrics (wordCount, passed, attempts, ...).
type Result struct {
	BookID  string                 `json:"bookId"`
	Chapter int                    `json:"chapter"`
	Stage   string                 `json:"stage"`
	Status  string                 `json:"status"`
	Body    string                 `json:"body,omitempty"`
	Extras  map[string]interface{} `json:"extras,omitempty"`
}

// resolve returns an LLM client + resolved model for the given service.
func (r *Runner) resolve(service string) (*llm.Client, *llm.ResolvedModel, error) {
	if r.ResolverFactory == nil {
		return nil, nil, ErrNoResolver
	}
	resolver := r.ResolverFactory()
	if resolver == nil {
		return nil, nil, ErrNoResolver
	}
	client, rm, err := resolver.Client(service)
	if err != nil {
		return nil, nil, err
	}
	if client == nil {
		return nil, nil, ErrNoClient
	}
	return client, rm, nil
}

// broadcast sends an SSE event for the current stage.
func (r *Runner) broadcast(bookID string, chapter int, stage, status string, extras map[string]interface{}) {
	if r.Broadcaster == nil {
		return
	}
	data := map[string]interface{}{
		"bookId":  bookID,
		"chapter": chapter,
		"stage":   stage,
		"status":  status,
	}
	for k, v := range extras {
		data[k] = v
	}
	r.Broadcaster.PublishEvent("pipeline:"+stage+":"+status, data)
}

// broadcastError sends a stage error event.
func (r *Runner) broadcastError(bookID string, chapter int, stage string, err error) {
	if r.Broadcaster == nil {
		return
	}
	r.Broadcaster.PublishEvent("pipeline:"+stage+":error", map[string]interface{}{
		"bookId":  bookID,
		"chapter": chapter,
		"stage":   stage,
		"error":   err.Error(),
	})
}

// chapterContext is the assembled book + foundation + state material
// passed to the LLM agents.
type chapterContext struct {
	Book        *model.BookConfig
	Foundation  map[string]string
	State       map[string]string
	Summaries   string
	PrevIntent  string
	TargetWords int
	Language    string
	Extra       string
}

// loadChapterContext gathers the book config, foundation files, current
// state, previous summaries, and (if any) the prior chapter intent.
func (r *Runner) loadChapterContext(bookID string, chapter int, extra string) (*chapterContext, error) {
	cfg, err := r.Books.LoadBookConfig(bookID)
	if err != nil {
		return nil, fmt.Errorf("load book config: %w", err)
	}
	cc := &chapterContext{
		Book:        cfg,
		Foundation:  map[string]string{},
		State:       map[string]string{},
		TargetWords: cfg.ChapterWordCount,
		Language:    cfg.Language,
		Extra:       extra,
	}
	for _, name := range []string{
		"story_bible.md", "book_rules.md", "author_intent.md",
		"character_matrix.md", "volume_outline.md",
	} {
		body, err := r.Truth.ReadTruth(bookID, strings.TrimSuffix(name, ".md"))
		if err == nil {
			cc.Foundation[name] = body
		}
	}
	for _, name := range []string{
		"current_focus.md", "current_state.md", "pending_hooks.md", "chapter_summaries.md",
	} {
		body, err := r.Truth.ReadTruth(bookID, strings.TrimSuffix(name, ".md"))
		if err == nil {
			cc.State[name] = body
		}
	}
	// Previous chapter summaries (consolidated).
	if body, err := r.Truth.ReadTruth(bookID, "chapter_summaries"); err == nil {
		cc.Summaries = body
	}
	// If chapter > 1, load the previous chapter's intent for continuity.
	if chapter > 1 {
		prev, err := r.Books.ReadIntent(bookID, chapter-1)
		if err == nil {
			cc.PrevIntent = prev
		}
	}
	return cc, nil
}

// buildPlannerPrompt assembles the user prompt for the planner agent.
func (r *Runner) buildPlannerPrompt(cc *chapterContext, chapter int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\n", cc.Book.Title))
	sb.WriteString(fmt.Sprintf("Genre: %s\n", cc.Book.Genre))
	sb.WriteString(fmt.Sprintf("Language: %s\n", cc.Language))
	sb.WriteString(fmt.Sprintf("Chapter: %d\n", chapter))
	if cc.Book.Blurb != "" {
		sb.WriteString("\n## Blurb\n" + cc.Book.Blurb + "\n")
	}
	if cc.Book.Brief != "" {
		sb.WriteString("\n## Brief\n" + cc.Book.Brief + "\n")
	}
	for _, k := range []string{
		"story_bible.md", "book_rules.md", "author_intent.md",
		"character_matrix.md", "volume_outline.md",
	} {
		if v, ok := cc.Foundation[k]; ok {
			sb.WriteString("\n## " + k + "\n" + truncate(v, r.MaxChapterContextChars) + "\n")
		}
	}
	for _, k := range []string{
		"current_focus.md", "current_state.md", "pending_hooks.md",
	} {
		if v, ok := cc.State[k]; ok {
			sb.WriteString("\n## " + k + "\n" + truncate(v, r.MaxChapterContextChars) + "\n")
		}
	}
	if cc.Summaries != "" {
		sb.WriteString("\n## Chapter Summaries\n" + truncate(cc.Summaries, r.MaxChapterContextChars) + "\n")
	}
	if cc.PrevIntent != "" {
		sb.WriteString("\n## Previous Chapter Intent\n" + truncate(cc.PrevIntent, r.MaxChapterContextChars/2) + "\n")
	}
	if cc.Extra != "" {
		sb.WriteString("\n## User Notes\n" + cc.Extra + "\n")
	}
	return sb.String()
}

// buildWriterPrompt assembles the user prompt for the writer agent.
func (r *Runner) buildWriterPrompt(cc *chapterContext, intent, contextJSON string, chapter int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s\nChapter: %d\n", cc.Book.Title, chapter))
	if intent != "" {
		sb.WriteString("\n## Chapter Intent\n" + intent + "\n")
	}
	if contextJSON != "" {
		sb.WriteString("\n## Composed Context\n" + contextJSON + "\n")
	}
	for _, k := range []string{
		"story_bible.md", "book_rules.md", "author_intent.md",
		"character_matrix.md",
	} {
		if v, ok := cc.Foundation[k]; ok {
			sb.WriteString("\n## " + k + "\n" + truncate(v, r.MaxChapterContextChars) + "\n")
		}
	}
	if cc.Summaries != "" {
		sb.WriteString("\n## Chapter Summaries\n" + truncate(cc.Summaries, r.MaxChapterContextChars) + "\n")
	}
	if cc.Extra != "" {
		sb.WriteString("\n## User Notes\n" + cc.Extra + "\n")
	}
	if cc.TargetWords > 0 {
		sb.WriteString(fmt.Sprintf("\nTarget word count: %d\n", cc.TargetWords))
	}
	return sb.String()
}

func truncate(s string, n int) string {
	if n <= 0 {
		n = 4000
	}
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n...(truncated)..."
}

// defaultChapter returns the next chapter number for the book.
func (r *Runner) defaultChapter(bookID string) (int, error) {
	next, err := r.Books.NextChapterNumber(bookID)
	if err != nil {
		return 0, err
	}
	if next < 1 {
		next = 1
	}
	return next, nil
}

// Plan runs the Planner agent and writes the intent markdown to
// books/<id>/story/runtime/chapter-XXXX.intent.md.
func (r *Runner) Plan(ctx context.Context, bookID string, chapter int, opts Options) (*Result, error) {
	if chapter < 1 {
		n, err := r.defaultChapter(bookID)
		if err != nil {
			return nil, err
		}
		chapter = n
	}
	r.broadcast(bookID, chapter, "plan", "start", nil)

	cc, err := r.loadChapterContext(bookID, chapter, opts.Context)
	if err != nil {
		r.broadcastError(bookID, chapter, "plan", err)
		return nil, err
	}
	client, _, err := r.resolve(opts.Service)
	if err != nil {
		r.broadcastError(bookID, chapter, "plan", err)
		return nil, err
	}
	intent, err := agents.NewPlanner().Run(ctx, client, r.buildPlannerPrompt(cc, chapter))
	if err != nil {
		r.broadcastError(bookID, chapter, "plan", err)
		return nil, fmt.Errorf("planner: %w", err)
	}
	if err := r.Books.WriteIntent(bookID, chapter, intent); err != nil {
		r.broadcastError(bookID, chapter, "plan", err)
		return nil, err
	}
	r.broadcast(bookID, chapter, "plan", "done", map[string]interface{}{"chars": len(intent)})
	return &Result{
		BookID:  bookID,
		Chapter: chapter,
		Stage:   "plan",
		Status:  "done",
		Body:    intent,
	}, nil
}

// Compose runs the Composer agent and writes the composed context to
// books/<id>/story/runtime/chapter-XXXX.context.json. The intent file
// is required; if missing, Compose returns an error.
func (r *Runner) Compose(ctx context.Context, bookID string, chapter int, opts Options) (*Result, error) {
	if chapter < 1 {
		n, err := r.defaultChapter(bookID)
		if err != nil {
			return nil, err
		}
		chapter = n
	}
	r.broadcast(bookID, chapter, "compose", "start", nil)

	intent, err := r.Books.ReadIntent(bookID, chapter)
	if err != nil {
		r.broadcastError(bookID, chapter, "compose", fmt.Errorf("read intent: %w", err))
		return nil, err
	}
	client, _, err := r.resolve(opts.Service)
	if err != nil {
		r.broadcastError(bookID, chapter, "compose", err)
		return nil, err
	}
	composed, err := agents.NewComposer().Run(ctx, client, intent)
	if err != nil {
		r.broadcastError(bookID, chapter, "compose", err)
		return nil, fmt.Errorf("composer: %w", err)
	}
	body, err := llm.ExtractJSON(composed)
	if err != nil {
		// Composer was supposed to wrap JSON in a fence, but if it
		// didn't, persist the raw reply rather than dropping it.
		body = composed
	}
	if err := r.Books.WriteComposeContext(bookID, chapter, body); err != nil {
		r.broadcastError(bookID, chapter, "compose", err)
		return nil, err
	}
	r.broadcast(bookID, chapter, "compose", "done", map[string]interface{}{"chars": len(body)})
	return &Result{
		BookID:  bookID,
		Chapter: chapter,
		Stage:   "compose",
		Status:  "done",
		Body:    body,
	}, nil
}

// Draft runs the Writer agent and writes the chapter file. If the
// intent / compose artifacts already exist, they are folded into the
// writer prompt; otherwise the writer is invoked with just the book
// context.
func (r *Runner) Draft(ctx context.Context, bookID string, chapter int, opts Options) (*Result, error) {
	if chapter < 1 {
		n, err := r.defaultChapter(bookID)
		if err != nil {
			return nil, err
		}
		chapter = n
	}
	r.broadcast(bookID, chapter, "draft", "start", nil)

	intent, _ := r.Books.ReadIntent(bookID, chapter)
	contextJSON, _ := r.Books.ReadComposeContext(bookID, chapter)
	cc, err := r.loadChapterContext(bookID, chapter, opts.Context)
	if err != nil {
		r.broadcastError(bookID, chapter, "draft", err)
		return nil, err
	}
	if opts.Words > 0 {
		cc.TargetWords = opts.Words
	}
	client, _, err := r.resolve(opts.Service)
	if err != nil {
		r.broadcastError(bookID, chapter, "draft", err)
		return nil, err
	}
	res, err := agents.NewWriter().Run(ctx, client,
		r.buildWriterPrompt(cc, intent, contextJSON, chapter),
		&agents.WriterOptions{WordsTarget: cc.TargetWords},
	)
	if err != nil {
		r.broadcastError(bookID, chapter, "draft", err)
		return nil, fmt.Errorf("writer: %w", err)
	}
	if err := r.Books.WriteChapter(bookID, chapter, res.Body); err != nil {
		r.broadcastError(bookID, chapter, "draft", err)
		return nil, err
	}
	now := nowISO()
	meta := model.ChapterMeta{
		Number:    chapter,
		Status:    model.ChapterStatusDraft,
		WordCount: res.WordCount,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := r.Books.AddOrUpdateChapter(bookID, meta); err != nil {
		r.broadcastError(bookID, chapter, "draft", err)
		return nil, err
	}
	r.broadcast(bookID, chapter, "draft", "done", map[string]interface{}{"wordCount": res.WordCount})
	return &Result{
		BookID:  bookID,
		Chapter: chapter,
		Stage:   "draft",
		Status:  "done",
		Body:    res.Body,
		Extras:  map[string]interface{}{"wordCount": res.WordCount},
	}, nil
}

// Audit runs the Auditor agent against the chapter file and persists
// the structured result to books/<id>/story/runtime/chapter-XXXX.audit.json.
func (r *Runner) Audit(ctx context.Context, bookID string, chapter int, opts Options) (*Result, error) {
	r.broadcast(bookID, chapter, "audit", "start", nil)

	body, err := r.Books.ReadChapter(bookID, chapter)
	if err != nil {
		r.broadcastError(bookID, chapter, "audit", fmt.Errorf("read chapter: %w", err))
		return nil, err
	}
	client, _, err := r.resolve(opts.Service)
	if err != nil {
		r.broadcastError(bookID, chapter, "audit", err)
		return nil, err
	}
	audit, err := agents.NewAuditor().Run(ctx, client, body, nil)
	if err != nil {
		r.broadcastError(bookID, chapter, "audit", err)
		return nil, fmt.Errorf("auditor: %w", err)
	}
	data, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		r.broadcastError(bookID, chapter, "audit", err)
		return nil, err
	}
	if err := r.Books.WriteAuditResult(bookID, chapter, data); err != nil {
		r.broadcastError(bookID, chapter, "audit", err)
		return nil, err
	}
	passed := auditPassed(audit)
	// Update chapter index status to "audited" regardless of pass/fail
	// so the UI can render the badge.
	now := nowISO()
	meta := model.ChapterMeta{
		Number:    chapter,
		Status:    model.ChapterStatusAuditing,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_ = r.Books.AddOrUpdateChapter(bookID, meta)
	r.broadcast(bookID, chapter, "audit", boolStatus(passed),
		map[string]interface{}{"passed": passed, "blockers": len(audit.Blockers)})
	return &Result{
		BookID:  bookID,
		Chapter: chapter,
		Stage:   "audit",
		Status:  boolStatus(passed),
		Body:    string(data),
		Extras: map[string]interface{}{
			"passed":   passed,
			"blockers": len(audit.Blockers),
		},
	}, nil
}

// Revise runs the Reviser agent using the latest audit result and
// overwrites the chapter file. The previous version is snapshotted
// under story/snapshots/ before the write.
func (r *Runner) Revise(ctx context.Context, bookID string, chapter int, opts Options) (*Result, error) {
	r.broadcast(bookID, chapter, "revise", "start", nil)

	body, err := r.Books.ReadChapter(bookID, chapter)
	if err != nil {
		r.broadcastError(bookID, chapter, "revise", fmt.Errorf("read chapter: %w", err))
		return nil, err
	}
	auditBytes, err := r.Books.ReadAuditResult(bookID, chapter)
	if err != nil {
		r.broadcastError(bookID, chapter, "revise", fmt.Errorf("read audit: %w", err))
		return nil, err
	}
	audit := &agents.AuditorResult{}
	if err := json.Unmarshal(auditBytes, audit); err != nil {
		r.broadcastError(bookID, chapter, "revise", fmt.Errorf("parse audit: %w", err))
		return nil, err
	}
	client, _, err := r.resolve(opts.Service)
	if err != nil {
		r.broadcastError(bookID, chapter, "revise", err)
		return nil, err
	}
	revised, err := agents.NewReviser().Run(ctx, client, body, agents.ReviseOptions{Audit: audit})
	if err != nil {
		r.broadcastError(bookID, chapter, "revise", err)
		return nil, fmt.Errorf("reviser: %w", err)
	}
	// Snapshot previous version, then write the revised one.
	if err := r.snapshotChapter(bookID, chapter, body); err != nil {
		// Non-fatal: log via SSE and continue.
		r.broadcastError(bookID, chapter, "revise", fmt.Errorf("snapshot: %w", err))
	}
	if err := r.Books.WriteChapter(bookID, chapter, revised); err != nil {
		r.broadcastError(bookID, chapter, "revise", err)
		return nil, err
	}
	words := agents.CountWords(revised)
	now := nowISO()
	meta := model.ChapterMeta{
		Number:    chapter,
		Status:    model.ChapterStatusRevising,
		WordCount: words,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_ = r.Books.AddOrUpdateChapter(bookID, meta)
	r.broadcast(bookID, chapter, "revise", "done", map[string]interface{}{"wordCount": words})
	return &Result{
		BookID:  bookID,
		Chapter: chapter,
		Stage:   "revise",
		Status:  "done",
		Body:    revised,
		Extras:  map[string]interface{}{"wordCount": words},
	}, nil
}

// WriteNext is the full state machine:
//
//	plan -> compose -> draft -> audit -> [ revise -> audit ]* (until
//	audit passes or MaxRetries is reached).
//
// The chapter number is taken from the next available slot.
func (r *Runner) WriteNext(ctx context.Context, bookID string, opts Options) (*Result, error) {
	chapter, err := r.defaultChapter(bookID)
	if err != nil {
		return nil, err
	}
	// Resolve MaxRetries from project config when not overridden.
	if opts.MaxRetries == 0 {
		if cfg, err := r.Project.LoadConfig(); err == nil && cfg != nil {
			opts.MaxRetries = cfg.Writing.ReviewRetries
		}
	}
	if opts.MaxRetries < 0 {
		opts.MaxRetries = 0
	}
	// Audit + Revise default to true if MaxRetries > 0.
	if opts.MaxRetries > 0 {
		opts.Audit = true
		opts.Revise = true
	}

	r.broadcast(bookID, chapter, "write-next", "start",
		map[string]interface{}{"maxRetries": opts.MaxRetries})

	if _, err := r.Plan(ctx, bookID, chapter, opts); err != nil {
		r.broadcastError(bookID, chapter, "write-next", err)
		return nil, err
	}
	if _, err := r.Compose(ctx, bookID, chapter, opts); err != nil {
		r.broadcastError(bookID, chapter, "write-next", err)
		return nil, err
	}
	if _, err := r.Draft(ctx, bookID, chapter, opts); err != nil {
		r.broadcastError(bookID, chapter, "write-next", err)
		return nil, err
	}
	if !opts.Audit {
		r.broadcast(bookID, chapter, "write-next", "done",
			map[string]interface{}{"status": "drafted", "attempts": 0})
		return &Result{BookID: bookID, Chapter: chapter, Stage: "write-next", Status: "drafted"}, nil
	}
	auditRes, err := r.Audit(ctx, bookID, chapter, opts)
	if err != nil {
		r.broadcastError(bookID, chapter, "write-next", err)
		return nil, err
	}
	attempts := 0
	for resultPassed(auditRes) == false && opts.Revise && attempts < opts.MaxRetries {
		attempts++
		r.broadcast(bookID, chapter, "write-next", "revising",
			map[string]interface{}{"attempt": attempts})
		if _, err := r.Revise(ctx, bookID, chapter, opts); err != nil {
			r.broadcastError(bookID, chapter, "write-next", err)
			return nil, err
		}
		auditRes, err = r.Audit(ctx, bookID, chapter, opts)
		if err != nil {
			r.broadcastError(bookID, chapter, "write-next", err)
			return nil, err
		}
	}
	final := "approved"
	if resultPassed(auditRes) == false {
		final = "audit-failed"
	}
	r.broadcast(bookID, chapter, "write-next", "done",
		map[string]interface{}{"status": final, "attempts": attempts})
	return &Result{
		BookID:  bookID,
		Chapter: chapter,
		Stage:   "write-next",
		Status:  final,
		Extras:  map[string]interface{}{"attempts": attempts},
	}, nil
}

// snapshotChapter saves the current chapter body to a snapshot file
// under story/snapshots/ so revisions are reversible.
func (r *Runner) snapshotChapter(bookID string, chapter int, body string) error {
	dir := r.Books.SnapshotsDir(bookID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("chapter-%04d.json", chapter)
	snap := map[string]interface{}{
		"chapter":    chapter,
		"body":       body,
		"snapshotAt": nowISO(),
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(filepath.Join(dir, name), data)
}

func auditPassed(a *agents.AuditorResult) bool {
	if a == nil {
		return false
	}
	if len(a.Blockers) > 0 {
		return false
	}
	if v, ok := a.Overall["pass"].(bool); ok {
		return v
	}
	// Models sometimes emit "pass": "true" — accept that too.
	if v, ok := a.Overall["pass"].(string); ok {
		return v == "true"
	}
	return true
}

func resultPassed(r *Result) bool {
	if r == nil {
		return false
	}
	if v, ok := r.Extras["passed"].(bool); ok {
		return v
	}
	return r.Status == "approved" || r.Status == "pass"
}

func boolStatus(passed bool) string {
	if passed {
		return "approved"
	}
	return "needs-revision"
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }
