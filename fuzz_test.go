package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzNormalizeVersion(f *testing.F) {
	for _, seed := range []string{"1.1", "v1.1.0", "v 1.1.", "1", "1.1-beta", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		version, err := normalizeVersion(raw)
		if err != nil {
			return
		}
		parts := strings.Split(version, ".")
		if len(parts) != 3 {
			t.Fatalf("unexpected normalized version for %q: %q", raw, version)
		}
	})
}

func FuzzValidCommunityPostURL(f *testing.F) {
	f.Add("https://vk.ru/wall-59626511_62336", int64(62336))
	f.Add("https://vk.com/wall-59626511_1", int64(1))
	f.Add("javascript:alert(1)", int64(1))
	f.Add("https://example.com/wall-59626511_62336", int64(62336))
	f.Fuzz(func(t *testing.T, raw string, postID int64) {
		if postID <= 0 {
			return
		}
		if validCommunityPostURL(raw, postID) {
			expected := "wall-59626511_"
			if !strings.Contains(raw, expected) {
				t.Fatalf("accepted URL without expected community path: %q", raw)
			}
		}
	})
}

func FuzzCleanCommunityPostText(f *testing.F) {
	for _, seed := range []string{
		"Обычная запись",
		"строка 1\\nстрока 2",
		"  много   пробелов  ",
		string([]byte{0xff, 0xfe, 'a'}),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		cleaned := cleanCommunityPostText(raw)
		if !utf8.ValidString(cleaned) {
			t.Fatalf("cleanCommunityPostText returned invalid UTF-8")
		}
		if len([]rune(cleaned)) > maxCommunityPostTextLength+1 {
			t.Fatalf("cleaned text is too long: %d runes", len([]rune(cleaned)))
		}
	})
}
