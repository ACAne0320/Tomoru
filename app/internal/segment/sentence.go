package segment

import (
	"strings"
	"unicode"
)

func firstSentenceEnd(text []rune) int {
	for i := 0; i < len(text); i++ {
		if !isEnder(text[i]) {
			continue
		}
		j := i + 1
		for j < len(text) && isEnder(text[j]) {
			j++
		}
		end := j
		for end < len(text) && isCloser(text[end]) {
			end++
		}
		if isDots(text[i:j]) && !periodEnds(text, i, j) {
			i = j - 1
			continue
		}
		return end
	}
	return -1
}

func periodEnds(text []rune, i, j int) bool {
	if j < len(text) && isSpeech(text[j]) {
		return false
	}
	k := j
	for k < len(text) && (unicode.IsSpace(text[k]) || isCloser(text[k])) {
		k++
	}
	if k < len(text) && unicode.IsLower(text[k]) {
		return false
	}
	if j-i > 1 {
		return true
	}
	w := i
	for w > 0 && (unicode.IsLetter(text[w-1]) || text[w-1] == '.') {
		w--
	}
	word := string(text[w:i])
	switch {
	case strings.Contains(word, "."):
		return false
	case len([]rune(word)) == 1 && unicode.IsUpper(text[w]):
		return false
	case abbreviations[strings.ToLower(word)]:
		return false
	}
	return true
}

var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true, "sr": true, "jr": true,
	"st": true, "mt": true, "vs": true, "vol": true, "fig": true, "approx": true, "dept": true,
	"inc": true, "ltd": true, "co": true, "corp": true,
	"jan": true, "feb": true, "apr": true, "aug": true, "sep": true, "sept": true, "oct": true,
	"nov": true, "dec": true,
}

func isEnder(r rune) bool { return strings.ContainsRune("。！？!?.…．", r) }

func isDots(rs []rune) bool {
	for _, r := range rs {
		if r != '.' && r != '…' {
			return false
		}
	}
	return true
}

func isCloser(r rune) bool { return strings.ContainsRune("」』）)]】〉》\"'”’", r) }
