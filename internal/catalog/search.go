package catalog

import "strings"

type Document struct {
	Literal string
	Stems   string
}
type Query struct {
	literal string
	stems   string
	words   []string
}

func normalizeSearch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "ё", "е")
	var builder strings.Builder
	builder.Grow(len(value))
	spacePending := false
	for _, r := range value {
		if (r >= 'а' && r <= 'я') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if spacePending && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			spacePending = false
			builder.WriteRune(r)
		} else {
			spacePending = true
		}
	}
	return strings.TrimSpace(builder.String())
}

var russianSearchSuffixes = []string{
	"иями", "ями", "ами", "ией", "иям", "ием", "иях", "ью", "ия", "ья",
	"ого", "ему", "ому", "ыми", "ими", "его", "ее", "ие", "ые", "ое",
	"ей", "ий", "ый", "ой", "ем", "им", "ым", "ом", "их", "ых",
	"ую", "юю", "ая", "яя", "ев", "ов", "ам", "ям", "ах", "ях",
	"а", "я", "ы", "и", "ь", "й", "у", "ю", "о", "е",
}

func russianSearchStem(word string) string {
	runes := []rune(word)
	if len(runes) <= 3 {
		return word
	}
	for _, suffix := range russianSearchSuffixes {
		suffixRunes := []rune(suffix)
		if len(runes)-len(suffixRunes) < 3 || !strings.HasSuffix(word, suffix) {
			continue
		}
		return string(runes[:len(runes)-len(suffixRunes)])
	}
	return word
}

func stemSearch(value string) string {
	normalized := normalizeSearch(value)
	if normalized == "" {
		return ""
	}
	words := strings.Fields(normalized)
	for i, word := range words {
		words[i] = russianSearchStem(word)
	}
	return strings.Join(words, " ")
}

func NewDocument(value string) Document {
	literal := normalizeSearch(value)
	return Document{Literal: literal, Stems: stemSearch(literal)}
}

func PrepareQuery(value string) Query {
	literal := normalizeSearch(value)
	stems := stemSearch(literal)
	return Query{literal: literal, stems: stems, words: strings.Fields(stems)}
}

func (q Query) Matches(document Document) bool {
	if q.literal == "" || strings.Contains(document.Literal, q.literal) {
		return true
	}
	if q.stems == "" {
		return false
	}
	if strings.Contains(document.Stems, q.stems) {
		return true
	}
	for _, word := range q.words {
		found := false
		for rest := document.Stems; rest != ""; {
			var token string
			token, rest, _ = strings.Cut(rest, " ")
			if token == word {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
