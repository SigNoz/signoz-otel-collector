package flatten

import "testing"

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
