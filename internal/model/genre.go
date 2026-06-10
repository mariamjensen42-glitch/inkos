package model

// GenreProfile is the parsed frontmatter of a genre markdown file.
type GenreProfile struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Language    string   `json:"language,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Rules       string   `json:"rules,omitempty"`
}

// ParsedGenreProfile wraps a GenreProfile with the raw markdown body.
type ParsedGenreProfile struct {
	Profile GenreProfile `json:"profile"`
	Body    string       `json:"body"`
}

// BookRules is the parsed content of story/book_rules.md.
type BookRules struct {
	Rules     string   `json:"rules"`
	Language  string   `json:"language,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

// ParsedBookRules wraps BookRules with the raw markdown body.
type ParsedBookRules struct {
	Rules BookRules `json:"rules"`
	Body  string    `json:"body"`
}
