package main

import (
	"net/http/httptest"
	"testing"
)

func benchmarkCatalog(b *testing.B, path string) {
	if err := ensureLoaded(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := httptest.NewRequest("GET", "http://wails.localhost"+path, nil)
		response := httptest.NewRecorder()
		handleItems(response, request)
		if response.Code != 200 {
			b.Fatal(response.Code)
		}
	}
}

func BenchmarkCatalogItems(b *testing.B) {
	benchmarkCatalog(b, "/api/items?sort=name&pageSize=24")
}

func BenchmarkCatalogSearch(b *testing.B) {
	benchmarkCatalog(b, "/api/items?q=999999999999&pageSize=24")
}
