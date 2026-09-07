package clickhousetracesexporter

import "testing"

func BenchmarkSpanFieldRowsDedupe(b *testing.B) {
	span := &SpanV3{
		Name:               "GET /users/{id}",
		Kind:               2,
		SpanKind:           "Server",
		StatusCodeString:   "Unset",
		HttpMethod:         "GET",
		HttpHost:           "api.example.com",
		HttpUrl:            "https://api.example.com/users/42",
		ResponseStatusCode: "200",
		DBName:             "orders",
		DBOperation:        "SELECT",
		IsRemote:           "false",
	}
	seen := make(map[spanFieldDedupeKey]struct{}, 16)
	rows := make([]spanFieldRow, 0, 16)

	b.ReportAllocs()
	for b.Loop() {
		rows = spanFieldRows(span, rows[:0])
		for _, row := range rows {
			seen[row.dedupeKey()] = struct{}{}
		}
	}
}
