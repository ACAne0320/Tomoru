package segment

import (
	"strings"
	"unicode"
)

// firstSentenceEnd returns where the first finished sentence of text ends, as a rune index just
// past its end mark and any closing quotes or brackets after it, or -1 when no sentence is
// finished yet.
//
// "。！？" and the like always end a sentence. A period ends one unless it is a decimal point or
// part of a word ("3.5", "e.g."), ends an abbreviation or an initial ("Mr.", "J."), or is followed
// by a lowercase word ("etc. and"), as ICU's sentence breaks with English exceptions do.
// Missing an end only delays a segment to the next pause; a false end splits a sentence, which
// translates worse, so the rules lean toward no end.
func firstSentenceEnd(text []rune) int {
	for i := 0; i < len(text); i++ {
		if !isEnder(text[i]) {
			continue
		}
		j := i + 1
		for j < len(text) && isEnder(text[j]) {
			j++ // "?!", "...", "……"
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

// periodEnds tells whether the dots text[i:j] end a sentence.
func periodEnds(text []rune, i, j int) bool {
	if j < len(text) && isSpeech(text[j]) {
		return false // a decimal point or a dot inside a word: "3.5", "U.S", "file.txt"
	}
	k := j
	for k < len(text) && (unicode.IsSpace(text[k]) || isCloser(text[k])) {
		k++
	}
	if k < len(text) && unicode.IsLower(text[k]) {
		return false // the sentence goes on: "etc. and", "well... maybe"
	}
	if j-i > 1 {
		return true // an ellipsis
	}
	// The word before the period.
	w := i
	for w > 0 && (unicode.IsLetter(text[w-1]) || text[w-1] == '.') {
		w--
	}
	word := string(text[w:i])
	switch {
	case strings.Contains(word, "."):
		return false // "e.g.", "U.S.", "p.m."
	case len([]rune(word)) == 1 && unicode.IsUpper(text[w]):
		return false // an initial: "J. K. Rowling"
	case abbreviations[strings.ToLower(word)]:
		return false
	}
	return true
}

// abbreviations are English words that a period ends without ending the sentence.
var abbreviations = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true, "sr": true, "jr": true,
	"st": true, "mt": true, "vs": true, "vol": true, "fig": true, "approx": true, "dept": true,
	"inc": true, "ltd": true, "co": true, "corp": true,
	"jan": true, "feb": true, "apr": true, "aug": true, "sep": true, "sept": true, "oct": true,
	"nov": true, "dec": true,
}

func isEnder(r rune) bool { return strings.ContainsRune("。！？!?.…．", r) }

// isDots tells whether rs are periods or ellipses alone, which may not end a sentence.
func isDots(rs []rune) bool {
	for _, r := range rs {
		if r != '.' && r != '…' {
			return false
		}
	}
	return true
}

// isCloser tells whether r closes a quote or a bracket, which stays with the sentence it ends.
func isCloser(r rune) bool { return strings.ContainsRune("」』）)]】〉》\"'”’", r) }
