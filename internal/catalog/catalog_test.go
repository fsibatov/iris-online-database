package catalog

import (
	"slices"
	"sort"
	"testing"
)

func TestSearchCompatibility(t *testing.T) {
	document := NewDocument("Редкий посох лесного хранителя № 120. Ёлка")
	for _, test := range []struct {
		query string
		want  bool
	}{
		{"", true},
		{"  !!!  ", true},
		{"ПОСОХ", true},
		{"посох редкий", true},
		{"редким посохом", true},
		{"елка", true},
		{"120", true},
		{"121", false},
		{"посох меч", false},
	} {
		t.Run(test.query, func(t *testing.T) {
			if got := PrepareQuery(test.query).Matches(document); got != test.want {
				t.Fatalf("Matches(%q) = %v, want %v", test.query, got, test.want)
			}
		})
	}
}

func TestNameOrder(t *testing.T) {
	names := []string{"", "#Меч", "123", "alpha", "Zebra", "ёж", "Жук", "ель", "Ель", " Ёлка ", "мечи", "меч"}
	prepared := make([]Name, len(names))
	for i, name := range names {
		prepared[i] = PrepareName(name)
	}
	sort.Slice(prepared, func(i, j int) bool { return prepared[i].Less(prepared[j]) })
	got := make([]string, len(prepared))
	for i, name := range prepared {
		got[i] = name.Text
		if name.Less(name) {
			t.Fatalf("name orders before itself: %q", name.Text)
		}
	}
	want := []string{"Ель", "ель", "ёж", "Ёлка", "Жук", "меч", "мечи", "alpha", "Zebra", "123", "#Меч", ""}
	if !slices.Equal(got, want) {
		t.Fatalf("name order = %q, want %q", got, want)
	}
}

func TestPreparedOperationsDoNotAllocate(t *testing.T) {
	query := PrepareQuery("посох редкий")
	document := NewDocument("Редкий посох хранителя")
	left, right := PrepareName("Ель"), PrepareName("Ёлка")
	allocations := testing.AllocsPerRun(100, func() {
		if !query.Matches(document) || !left.Less(right) {
			t.Fatal("prepared operation changed its result")
		}
	})
	if allocations != 0 {
		t.Fatalf("prepared operations allocate: %g allocations", allocations)
	}
}
