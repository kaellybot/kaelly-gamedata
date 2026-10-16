package mappers

import (
	"html"
	"strings"
)

// NormalizeText strips HTML tags and entities, unifies apostrophes and spaces, and
// collapses whitespace, so a Krosmoz text can be compared to a client label.
func NormalizeText(text string) string {
	var builder strings.Builder
	inTag := false
	for _, char := range text {
		switch {
		case char == '<':
			inTag = true
		case char == '>' && inTag:
			inTag = false
		case !inTag:
			builder.WriteRune(char)
		}
	}

	replacer := strings.NewReplacer(" ", " ", " ", " ", "’", "'")
	cleaned := replacer.Replace(html.UnescapeString(builder.String()))
	return strings.Join(strings.Fields(cleaned), " ")
}
