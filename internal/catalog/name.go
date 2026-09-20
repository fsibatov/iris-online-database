package catalog

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Name struct {
	Text   string
	Class  int
	folded []rune
}

func isRussianCatalogLetter(r rune) bool {
	r = unicode.ToLower(r)
	return (r >= 'а' && r <= 'я') || r == 'ё'
}

func russianCatalogLetterOrder(r rune) int {
	r = unicode.ToLower(r)
	const alphabet = "абвгдеёжзийклмнопрстуфхцчшщъыьэюя"
	for index, letter := range []rune(alphabet) {
		if r == letter {
			return index
		}
	}
	return -1
}

func catalogNameClass(name string) int {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 4
	}
	first, _ := utf8.DecodeRuneInString(trimmed)
	if isRussianCatalogLetter(first) {
		return 0
	}
	if unicode.IsLetter(first) {
		return 1
	}
	if unicode.IsDigit(first) {
		return 2
	}
	return 3
}

func PrepareName(value string) Name {
	text := strings.TrimSpace(value)
	return Name{Text: text, Class: catalogNameClass(text), folded: []rune(strings.ToLower(text))}
}

func (left Name) Less(right Name) bool {
	if left.Class != right.Class {
		return left.Class < right.Class
	}
	if left.Class == 4 {
		return false
	}
	if strings.EqualFold(left.Text, right.Text) {
		return left.Text < right.Text
	}
	limit := min(len(left.folded), len(right.folded))
	for i := 0; i < limit; i++ {
		a, b := left.folded[i], right.folded[i]
		if a == b {
			continue
		}
		ar, br := russianCatalogLetterOrder(a), russianCatalogLetterOrder(b)
		if ar >= 0 && br >= 0 {
			return ar < br
		}
		return a < b
	}
	return len(left.folded) < len(right.folded)
}
