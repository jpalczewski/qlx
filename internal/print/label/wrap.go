package label

import (
	"strings"

	"golang.org/x/image/font"
)

// wrapText splits text into lines that fit within maxWidth pixels.
// Word boundaries are preferred; falls back to character breaking for long words.
func wrapText(text string, face font.Face, maxWidth int) []string {
	if text == "" {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		test := current + " " + word
		w := font.MeasureString(face, test).Ceil()
		if w <= maxWidth {
			current = test
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	lines = append(lines, current)

	// Break any lines that are still too long at character boundaries
	var result []string
	for _, line := range lines {
		w := font.MeasureString(face, line).Ceil()
		if w <= maxWidth {
			result = append(result, line)
			continue
		}
		result = append(result, breakLongLine(line, face, maxWidth)...)
	}
	return result
}

// breakLongLine breaks a single line at character boundaries to fit within maxWidth.
func breakLongLine(line string, face font.Face, maxWidth int) []string {
	var result []string
	runes := []rune(line)
	start := 0
	for start < len(runes) {
		end := start + 1
		for end <= len(runes) {
			w := font.MeasureString(face, string(runes[start:end])).Ceil()
			if w > maxWidth && end > start+1 {
				end--
				break
			}
			end++
		}
		if end > len(runes) {
			end = len(runes)
		}
		result = append(result, string(runes[start:end]))
		start = end
	}
	return result
}
