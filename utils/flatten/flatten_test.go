package flatten

import (
	"encoding/json"
	"testing"
)

func TestFlattenJSON(t *testing.T) {
	type args struct {
		data   map[string]interface{}
		prefix string
	}
	tests := []struct {
		name   string
		args   args
		result map[string]interface{}
	}{
		{
			name: "FlattenJSON",
			args: args{
				data: map[string]interface{}{
					"hello": "world",
					"user": map[string]interface{}{
						"name": "Alice",
						"age":  30,
						"address": map[string]interface{}{
							"city": "Wonderland",
							"zip":  "12345",
						},
					},
					"items": []interface{}{
						map[string]interface{}{
							"name":  "item1",
							"price": 10,
						},
						map[string]interface{}{
							"name":  "item2",
							"price": 20,
						},
					},
				},
				prefix: "",
			},
			result: map[string]interface{}{
				"hello":             "world",
				"user.name":         "Alice",
				"user.age":          30.0,
				"user.address.city": "Wonderland",
				"user.address.zip":  "12345",
				"items.0.name":      "item1",
				"items.0.price":     10.0,
				"items.1.name":      "item2",
				"items.1.price":     20.0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlattenJSON(tt.args.data, tt.args.prefix)
			if len(got) != len(tt.result) {
				t.Errorf("FlattenJSON() got %v entries, want %v entries", len(got), len(tt.result))
				return
			}
			for k, v := range tt.result {
				if gotV, ok := got[k]; !ok {
					t.Errorf("FlattenJSON() missing key %v", k)
				} else if gotV != v {
					t.Errorf("FlattenJSON() got %v = %v, want %v", k, gotV, v)
				}
			}
		})
	}
}

func TestConvertJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name: "Simple dotted keys",
			input: `{
				"user.name": "Alice",
				"user.age": 30,
				"metrics.cpu.usage": 85
			}`,
			want:    `{"user":{"name":"Alice","age":30},"metrics":{"cpu":{"usage":85}}}`,
			wantErr: false,
		},
		{
			name: "Mixed dotted and regular keys",
			input: `{
				"user.name": "Alice",
				"user.age": 30,
				"status": "active",
				"logs.error.message": "Something went wrong"
			}`,
			want:    `{"user":{"name":"Alice","age":30},"status":"active","logs":{"error":{"message":"Something went wrong"}}}`,
			wantErr: false,
		},
		{
			name: "Nested objects with dotted keys",
			input: `{
				"user": {
					"profile.name": "Alice Smith",
					"profile.email": "alice@example.com"
				},
				"activity": {
					"session.id": "abc123",
					"session.duration": 3600
				}
			}`,
			want:    `{"user":{"profile":{"name":"Alice Smith","email":"alice@example.com"}},"activity":{"session":{"id":"abc123","duration":3600}}}`,
			wantErr: false,
		},
		{
			name: "Arrays with dotted keys",
			input: `{
				"logs": [
					{ "event.type": "click", "meta.device": "mobile" },
					{ "event.type": "scroll", "meta.device": "desktop" }
				]
			}`,
			want:    `{"logs":[{"event":{"type":"click"},"meta":{"device":"mobile"}},{"event":{"type":"scroll"},"meta":{"device":"desktop"}}]}`,
			wantErr: false,
		},
		{
			name: "Overlapping dotted keys",
			input: `{
				"user.name": "Alice",
				"user": {
					"age": 30
				}
			}`,
			want:    `{"user":{"name":"Alice","age":30}}`,
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			input:   `{ invalid json }`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertJSON(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Compare JSON objects instead of strings to handle formatting differences
			var expectedObj, outputObj interface{}
			if err := json.Unmarshal([]byte(tt.want), &expectedObj); err != nil {
				t.Fatalf("unmarshal expected failed: %v", err)
			}
			if err := json.Unmarshal([]byte(got), &outputObj); err != nil {
				t.Fatalf("unmarshal output failed: %v", err)
			}

			if !deepEqual(outputObj, expectedObj) {
				t.Errorf("ConvertJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func deepEqual(a, b interface{}) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

func BenchmarkConvertJSON(b *testing.B) {

	notJSON := `hello word`

	smallJSON := `{
		"user.name": "Alice",
		"user.age": 30,
		"status": "active"
	}`

	mediumJSON := `{
		"user.name": "Alice",
		"user.age": 30,
		"user.email": "alice@example.com",
		"user.address.city": "Wonderland",
		"user.address.zip": "12345",
		"metrics.cpu.usage": 85,
		"metrics.memory.used": 1024,
		"logs.error.message": "Something went wrong",
		"logs.error.code": 500,
		"activity.session.id": "abc123",
		"activity.session.duration": 3600
	}`

	largeJSON := `{
		"user.profile.name": "Alice Smith",
		"user.profile.email": "alice@example.com",
		"user.profile.age": 30,
		"user.profile.address.city": "Wonderland",
		"user.profile.address.zip": "12345",
		"user.profile.address.country": "Fantasy",
		"user.settings.theme": "dark",
		"user.settings.language": "en",
		"user.settings.notifications.email": true,
		"user.settings.notifications.push": false,
		"metrics.system.cpu.usage": 85,
		"metrics.system.cpu.temperature": 65,
		"metrics.system.memory.used": 1024,
		"metrics.system.memory.total": 2048,
		"metrics.system.disk.used": 500,
		"metrics.system.disk.total": 1000,
		"metrics.network.bytes.sent": 1000000,
		"metrics.network.bytes.received": 2000000,
		"logs.application.error.message": "Something went wrong",
		"logs.application.error.code": 500,
		"logs.application.error.stack": "stack trace here",
		"logs.application.warning.message": "Warning message",
		"logs.application.warning.code": 300,
		"logs.system.info.message": "System info",
		"logs.system.info.level": "info",
		"activity.session.id": "abc123",
		"activity.session.duration": 3600,
		"activity.session.start_time": "2023-01-01T00:00:00Z",
		"activity.session.end_time": "2023-01-01T01:00:00Z",
		"activity.events": [
			{"event.type": "click", "event.timestamp": "2023-01-01T00:01:00Z"},
			{"event.type": "scroll", "event.timestamp": "2023-01-01T00:02:00Z"},
			{"event.type": "input", "event.timestamp": "2023-01-01T00:03:00Z"}
		]
	}`

	veryLargeJSON := `{
		"data": {
			"users": [
				{"user.profile.name": "User1", "user.profile.email": "user1@example.com", "user.profile.age": 25, "user.settings.theme": "light"},
				{"user.profile.name": "User2", "user.profile.email": "user2@example.com", "user.profile.age": 30, "user.settings.theme": "dark"},
				{"user.profile.name": "User3", "user.profile.email": "user3@example.com", "user.profile.age": 35, "user.settings.theme": "light"},
				{"user.profile.name": "User4", "user.profile.email": "user4@example.com", "user.profile.age": 28, "user.settings.theme": "dark"},
				{"user.profile.name": "User5", "user.profile.email": "user5@example.com", "user.profile.age": 32, "user.settings.theme": "light"},
				{"user.profile.name": "User6", "user.profile.email": "user6@example.com", "user.profile.age": 27, "user.settings.theme": "dark"},
				{"user.profile.name": "User7", "user.profile.email": "user7@example.com", "user.profile.age": 33, "user.settings.theme": "light"},
				{"user.profile.name": "User8", "user.profile.email": "user8@example.com", "user.profile.age": 29, "user.settings.theme": "dark"},
				{"user.profile.name": "User9", "user.profile.email": "user9@example.com", "user.profile.age": 31, "user.settings.theme": "light"},
				{"user.profile.name": "User10", "user.profile.email": "user10@example.com", "user.profile.age": 26, "user.settings.theme": "dark"}
			],
			"metrics": {
				"system.cpu.usage": 85,
				"system.memory.used": 1024,
				"system.disk.used": 500,
				"network.bytes.sent": 1000000,
				"network.bytes.received": 2000000,
				"application.errors.count": 5,
				"application.warnings.count": 10,
				"application.info.count": 50,
				"performance.page_load.time": 2500,
				"performance.resources.total_count": 25,
				"security.ssl.certificate.valid": true,
				"security.headers.content_security_policy": "default-src 'self'"
			},
			"logs": [
				{"event.type": "click", "event.timestamp": "2023-01-01T00:01:00Z", "event.element": "button"},
				{"event.type": "scroll", "event.timestamp": "2023-01-01T00:02:00Z", "event.element": "page"},
				{"event.type": "input", "event.timestamp": "2023-01-01T00:03:00Z", "event.element": "textbox"},
				{"event.type": "hover", "event.timestamp": "2023-01-01T00:04:00Z", "event.element": "link"},
				{"event.type": "submit", "event.timestamp": "2023-01-01T00:05:00Z", "event.element": "form"},
				{"event.type": "click", "event.timestamp": "2023-01-01T00:06:00Z", "event.element": "link"},
				{"event.type": "scroll", "event.timestamp": "2023-01-01T00:07:00Z", "event.element": "page"},
				{"event.type": "input", "event.timestamp": "2023-01-01T00:08:00Z", "event.element": "textbox"},
				{"event.type": "hover", "event.timestamp": "2023-01-01T00:09:00Z", "event.element": "button"},
				{"event.type": "submit", "event.timestamp": "2023-01-01T00:10:00Z", "event.element": "form"}
			]
		}
	}`

	benchmarks := []struct {
		name string
		json string
	}{
		{"NotJSON", notJSON},
		{"Small", smallJSON},
		{"Medium", mediumJSON},
		{"Large", largeJSON},
		{"VeryLarge", veryLargeJSON},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ConvertJSON(bm.json)
			}
		})
	}
}

func BenchmarkJSONSizeComparison(b *testing.B) {
	// Test different JSON sizes to see performance characteristics
	sizes := []struct {
		name string
		json string
	}{
		{"Small", `{"user.name": "Alice", "user.age": 30}`},
		{"Medium", `{"user.name": "Alice", "user.age": 30, "user.email": "alice@example.com", "metrics.cpu.usage": 85, "metrics.memory.used": 1024, "logs.error.message": "Error", "logs.error.code": 500, "activity.session.id": "abc123", "activity.session.duration": 3600}`},
		{"Large", `{"user.profile.name": "Alice Smith", "user.profile.email": "alice@example.com", "user.profile.age": 30, "user.profile.address.city": "Wonderland", "user.profile.address.zip": "12345", "user.profile.address.country": "Fantasy", "user.settings.theme": "dark", "user.settings.language": "en", "user.settings.notifications.email": true, "user.settings.notifications.push": false, "metrics.system.cpu.usage": 85, "metrics.system.cpu.temperature": 65, "metrics.system.memory.used": 1024, "metrics.system.memory.total": 2048, "metrics.system.disk.used": 500, "metrics.system.disk.total": 1000, "metrics.network.bytes.sent": 1000000, "metrics.network.bytes.received": 2000000, "logs.application.error.message": "Something went wrong", "logs.application.error.code": 500, "logs.application.warning.message": "Warning message", "logs.application.warning.code": 300, "logs.system.info.message": "System info", "logs.system.info.level": "info", "activity.session.id": "abc123", "activity.session.duration": 3600, "activity.session.start_time": "2023-01-01T00:00:00Z", "activity.session.end_time": "2023-01-01T01:00:00Z"}`},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ConvertJSON(size.json)
			}
		})
	}
}
