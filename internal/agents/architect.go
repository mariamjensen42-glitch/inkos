// Package agents provides LLM-powered autonomous agents for InkOS.
package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/narcooo/inkos/internal/llm"
)

// LLMCaller abstracts the LLM call so tests can inject mocks.
type LLMCaller interface {
	ChatOnce(ctx context.Context, service, system, user string, opts *llm.Request) (string, error)
}

// ArchitectInput holds the user-provided book creation parameters.
type ArchitectInput struct {
	Title           string
	Genre           string
	Language        string
	Platform        string
	Blurb           string
	Brief           string
	ChapterWordCount int
	TargetChapters  int
}

// ArchitectOutputFile represents one generated foundation file.
type ArchitectOutputFile struct {
	Name string // e.g. "story_bible.md"
	Body string
}

// ArchitectProgressFunc is called during generation. The caller can use
// the phase and optional file name to drive a progress UI.
// phase values: "start", "file" (per-file, name set), "done", "error"
type ArchitectProgressFunc func(phase string, file string, err error)

// Architect generates the 5 book-foundation markdown files via an LLM call.
// It writes results to bookDir/story/ and returns the list of generated files.
type Architect struct {
	LLM LLMCaller
}

// NewArchitect returns an Architect ready to run.
func NewArchitect(caller LLMCaller) *Architect {
	return &Architect{LLM: caller}
}

// Run executes the architect agent. It calls the LLM up to 2 times
// (1 attempt + 1 retry), writes the 5 output files, and invokes
// the progress callback at each stage.
func (a *Architect) Run(ctx context.Context, input ArchitectInput, storyDir string, onProgress ArchitectProgressFunc) ([]ArchitectOutputFile, error) {
	if onProgress == nil {
		onProgress = func(phase, file string, err error) {}
	}

	onProgress("start", "", nil)

	systemPrompt := architectSystemPrompt()
	userPrompt := architectUserPrompt(input)

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			onProgress("retry", "", nil)
		}
		onProgress("planning", "", nil)

		text, err := a.LLM.ChatOnce(ctx, "", systemPrompt, userPrompt, nil)
		if err != nil {
			lastErr = fmt.Errorf("architect LLM call (attempt %d): %w", attempt+1, err)
			continue
		}

		files, err := parseArchitectOutput(text)
		if err != nil {
			lastErr = fmt.Errorf("architect parse (attempt %d): %w", attempt+1, err)
			continue
		}

		if len(files) == 0 {
			lastErr = fmt.Errorf("architect produced no output files (attempt %d)", attempt+1)
			continue
		}

		// Write each file to disk.
		for _, f := range files {
			onProgress("file", f.Name, nil)
			path := filepath.Join(storyDir, f.Name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				lastErr = fmt.Errorf("architect mkdir: %w", err)
				onProgress("error", "", lastErr)
				return nil, lastErr
			}
			tmp := path + ".tmp"
			if err := os.WriteFile(tmp, []byte(f.Body), 0o644); err != nil {
				lastErr = fmt.Errorf("architect write %s: %w", f.Name, err)
				onProgress("error", "", lastErr)
				return nil, lastErr
			}
			if err := os.Rename(tmp, path); err != nil {
				lastErr = fmt.Errorf("architect rename %s: %w", f.Name, err)
				onProgress("error", "", lastErr)
				return nil, lastErr
			}
		}

		onProgress("done", "", nil)
		return files, nil
	}

	onProgress("error", "", lastErr)
	return nil, lastErr
}

// architectSystemPrompt is the system message that instructs the LLM how
// to format its output using the === FILE: name.md === delimiter.
func architectSystemPrompt() string {
	return `You are an expert book architect and narrative designer. Your task is to create foundational documents for a new novel.

You MUST output exactly 5 files, each separated by the delimiter "=== FILE: filename.md ===".
The delimiter line MUST appear alone on its own line, exactly as shown.
Do NOT include any text before the first delimiter or after the last file's content.

The 5 files you must produce are:

1. story_bible.md — World-building bible. Include: setting overview, magic/power system, key locations, factions/organizations, historical timeline, cultural notes.

2. book_rules.md — Writing rules and conventions. Include: tone/style guide, POV rules, chapter structure conventions, forbidden elements, must-have elements, pacing guidelines.

3. author_intent.md — Author's creative intent. Include: core theme, emotional journey, target reader experience, what makes this story unique, key messages.

4. current_focus.md — Current narrative focus. Include: immediate story goals, active plot threads, character arcs in focus, upcoming milestones.

5. character_matrix.md — Character roster. Include: protagonist(s), antagonist(s), supporting cast, each with name, role, motivation, arc summary, relationships.

IMPORTANT formatting rules:
- The delimiter "=== FILE: name.md ===" MUST be on a line by itself, with exactly one blank line after it.
- Use proper Markdown formatting inside each file.
- Be thorough and detailed — each file should be at least 200 words.
- Write in the language specified by the user prompt.
- Do NOT wrap file content in code fences.`
}

// architectUserPrompt builds the user message containing all book metadata.
func architectUserPrompt(input ArchitectInput) string {
	var b strings.Builder
	b.WriteString("Please create the 5 foundation files for the following novel:\n\n")
	b.WriteString(fmt.Sprintf("Title: %s\n", input.Title))
	if input.Genre != "" {
		b.WriteString(fmt.Sprintf("Genre: %s\n", input.Genre))
	}
	if input.Language != "" {
		b.WriteString(fmt.Sprintf("Language: %s\n", input.Language))
	}
	if input.Platform != "" {
		b.WriteString(fmt.Sprintf("Target Platform: %s\n", input.Platform))
	}
	if input.Blurb != "" {
		b.WriteString(fmt.Sprintf("\nBlurb:\n%s\n", input.Blurb))
	}
	if input.Brief != "" {
		b.WriteString(fmt.Sprintf("\nAuthor's Brief:\n%s\n", input.Brief))
	}
	if input.ChapterWordCount > 0 {
		b.WriteString(fmt.Sprintf("\nTarget chapter length: ~%d words\n", input.ChapterWordCount))
	}
	if input.TargetChapters > 0 {
		b.WriteString(fmt.Sprintf("Target total chapters: %d\n", input.TargetChapters))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	b.WriteString(fmt.Sprintf("\nGenerated at: %s", now))
	return b.String()
}

// parseArchitectOutput splits the LLM response into per-file chunks
// using the === FILE: name.md === delimiter.
func parseArchitectOutput(text string) ([]ArchitectOutputFile, error) {
	// Normalize line endings.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)

	// Split on === FILE: name.md ===
	parts := splitByFileDelimiter(text)
	if len(parts) == 0 {
		return nil, fmt.Errorf("no file delimiters found in architect output")
	}

	// Collect and sort by name for deterministic output.
	var files []ArchitectOutputFile
	for name, body := range parts {
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		files = append(files, ArchitectOutputFile{
			Name: name,
			Body: body,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("architect output contained empty files")
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

// splitByFileDelimiter parses text looking for "=== FILE: name.md ===" lines
// and returns a map of filename -> content.
func splitByFileDelimiter(text string) map[string]string {
	result := map[string]string{}

	lines := strings.Split(text, "\n")
	var currentFile string
	var currentBody strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line is a file delimiter.
		if strings.HasPrefix(trimmed, "=== FILE:") && strings.HasSuffix(trimmed, "===") {
			// Extract filename from "=== FILE: name.md ==="
			inner := strings.TrimPrefix(trimmed, "=== FILE:")
			inner = strings.TrimSuffix(inner, "===")
			inner = strings.TrimSpace(inner)

			// Save previous file if any.
			if currentFile != "" {
				result[currentFile] = currentBody.String()
			}

			currentFile = inner
			currentBody.Reset()
			continue
		}

		if currentFile != "" {
			currentBody.WriteString(line)
			currentBody.WriteByte('\n')
		}
	}

	// Save the last file.
	if currentFile != "" {
		content := strings.TrimRight(currentBody.String(), "\n")
		if content != "" {
			result[currentFile] = content
		}
	}

	return result
}