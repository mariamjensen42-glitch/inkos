package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/narcooo/inkos/internal/llm"
)

// mockLLMCaller implements LLMCaller for testing.
type mockLLMCaller struct {
	responses []string
	callCount int
}

func (m *mockLLMCaller) ChatOnce(ctx context.Context, service, system, user string, opts *llm.Request) (string, error) {
	if m.callCount >= len(m.responses) {
		return m.responses[len(m.responses)-1], nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func validArchitectOutput() string {
	return `=== FILE: story_bible.md ===

# Story Bible

## Setting Overview
A vast cultivation world where spiritual energy flows through all things.

## Magic System
The power system is based on Qi cultivation through 9 major realms.

## Key Locations
- Azure Cloud Sect: The protagonist's starting sect
- Demon Abyss: A forbidden zone filled with ancient beasts
- Celestial Capital: The seat of the immortal emperor

=== FILE: book_rules.md ===

# Book Rules

## Tone
Epic yet personal, balancing grand-scale conflicts with intimate character moments.

## POV
Third person limited, primarily following the protagonist.

## Chapter Structure
Each chapter should end with a cliffhanger or revelation.

=== FILE: author_intent.md ===

# Author Intent

## Core Theme
Power comes with responsibility. The journey matters more than the destination.

## Emotional Journey
Readers should feel the protagonist's struggle, growth, and triumph.

=== FILE: current_focus.md ===

# Current Focus

## Immediate Goals
Introduce the protagonist's humble beginnings and the inciting incident.

## Active Plot Threads
- The mysterious inheritance
- Rivalry with the sect's top disciple

=== FILE: character_matrix.md ===

# Character Matrix

## Protagonist
- **Name**: Lin Feng
- **Role**: Underdog cultivator
- **Motivation**: To protect those he loves and uncover the truth about his origins
- **Arc**: From weak to strong, discovering his hidden heritage

## Antagonist
- **Name**: Elder Mo
- **Role**: Corrupt sect elder
- **Motivation**: Greed for power and immortality

## Supporting Cast
- **Name**: Liu Xue
- **Role**: Childhood friend and love interest
- **Arc**: Discovers her own hidden powers`
}

func TestParseArchitectOutput(t *testing.T) {
	text := validArchitectOutput()
	files, err := parseArchitectOutput(text)
	if err != nil {
		t.Fatalf("parseArchitectOutput failed: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("expected 5 files, got %d", len(files))
	}
	expectedNames := []string{
		"author_intent.md",
		"book_rules.md",
		"character_matrix.md",
		"current_focus.md",
		"story_bible.md",
	}
	for i, f := range files {
		if f.Name != expectedNames[i] {
			t.Errorf("file %d: expected %q, got %q", i, expectedNames[i], f.Name)
		}
		if strings.TrimSpace(f.Body) == "" {
			t.Errorf("file %s has empty body", f.Name)
		}
	}
}

func TestParseArchitectOutput_Empty(t *testing.T) {
	_, err := parseArchitectOutput("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseArchitectOutput_NoDelimiters(t *testing.T) {
	_, err := parseArchitectOutput("just some plain text without delimiters")
	if err == nil {
		t.Fatal("expected error for no delimiters")
	}
}

func TestParseArchitectOutput_SingleFile(t *testing.T) {
	text := "=== FILE: test.md ===\n\n# Hello World\n"
	files, err := parseArchitectOutput(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "test.md" {
		t.Errorf("expected test.md, got %q", files[0].Name)
	}
}

func TestArchitectRun_WritesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	storyDir := filepath.Join(tmpDir, "story")
	if err := os.MkdirAll(storyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mock := &mockLLMCaller{
		responses: []string{validArchitectOutput()},
	}
	arch := NewArchitect(mock)

	phases := []string{}
	onProgress := func(phase, file string, err error) {
		phases = append(phases, phase)
	}

	input := ArchitectInput{
		Title:           "Test Novel",
		Genre:           "xianxia",
		Language:        "zh",
		Platform:        "qidian",
		Blurb:           "A test story",
		Brief:           "Just a test",
		ChapterWordCount: 2000,
		TargetChapters:  100,
	}

	files, err := arch.Run(context.Background(), input, storyDir, onProgress)
	if err != nil {
		t.Fatalf("architect.Run failed: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("expected 5 files, got %d", len(files))
	}

	// Verify all expected phases appeared.
	hasStart := false
	hasDone := false
	hasFile := false
	for _, p := range phases {
		switch p {
		case "start":
			hasStart = true
		case "done":
			hasDone = true
		case "file":
			hasFile = true
		}
	}
	if !hasStart {
		t.Error("missing 'start' phase")
	}
	if !hasDone {
		t.Error("missing 'done' phase")
	}
	if !hasFile {
		t.Error("missing 'file' phase")
	}

	// Verify files are on disk.
	expectedFileNames := []string{
		"story_bible.md",
		"book_rules.md",
		"author_intent.md",
		"current_focus.md",
		"character_matrix.md",
	}
	for _, name := range expectedFileNames {
		path := filepath.Join(storyDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("file %s not found on disk: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("file %s is empty", name)
		}
	}
}

func TestArchitectRun_RetryOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	storyDir := filepath.Join(tmpDir, "story")
	if err := os.MkdirAll(storyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// First response is invalid, second is valid.
	mock := &mockLLMCaller{
		responses: []string{
			"no delimiters here, just garbage",
			validArchitectOutput(),
		},
	}
	arch := NewArchitect(mock)

	files, err := arch.Run(context.Background(), ArchitectInput{
		Title: "Retry Test",
	}, storyDir, nil)
	if err != nil {
		t.Fatalf("architect.Run with retry failed: %v", err)
	}
	if len(files) != 5 {
		t.Fatalf("expected 5 files after retry, got %d", len(files))
	}
	if mock.callCount != 2 {
		t.Errorf("expected 2 LLM calls, got %d", mock.callCount)
	}
}

func TestArchitectRun_ErrorPhase(t *testing.T) {
	tmpDir := t.TempDir()
	storyDir := filepath.Join(tmpDir, "story")
	if err := os.MkdirAll(storyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mock := &mockLLMCaller{
		responses: []string{"bad output", "also bad output"},
	}
	arch := NewArchitect(mock)

	phases := []string{}
	onProgress := func(phase, file string, err error) {
		phases = append(phases, phase)
	}

	_, err := arch.Run(context.Background(), ArchitectInput{
		Title: "Error Test",
	}, storyDir, onProgress)
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}

	hasError := false
	for _, p := range phases {
		if p == "error" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected 'error' phase in progress callbacks")
	}
}

func TestArchitectUserPrompt(t *testing.T) {
	input := ArchitectInput{
		Title:           "Test Book",
		Genre:           "xianxia",
		Language:        "zh",
		Platform:        "qidian",
		Blurb:           "An epic tale",
		Brief:           "Write something great",
		ChapterWordCount: 3000,
		TargetChapters:  50,
	}
	prompt := architectUserPrompt(input)
	if !strings.Contains(prompt, "Test Book") {
		t.Error("prompt missing title")
	}
	if !strings.Contains(prompt, "xianxia") {
		t.Error("prompt missing genre")
	}
	if !strings.Contains(prompt, "An epic tale") {
		t.Error("prompt missing blurb")
	}
	if !strings.Contains(prompt, "Write something great") {
		t.Error("prompt missing brief")
	}
	if !strings.Contains(prompt, "3000") {
		t.Error("prompt missing chapter word count")
	}
	if !strings.Contains(prompt, "50") {
		t.Error("prompt missing target chapters")
	}
}

func TestArchitectSystemPrompt(t *testing.T) {
	prompt := architectSystemPrompt()
	if !strings.Contains(prompt, "=== FILE:") {
		t.Error("system prompt should mention the delimiter format")
	}
	if !strings.Contains(prompt, "story_bible.md") {
		t.Error("system prompt should list story_bible.md")
	}
}