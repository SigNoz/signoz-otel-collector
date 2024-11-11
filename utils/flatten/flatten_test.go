package flatten

import "testing"

func TestFlattenJSON(t *testing.T) {
	type args struct {
		data   map[string]interface{}
		prefix string
		result map[string]string
	}
	tests := []struct {
		name string
		args args
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
				result: map[string]string{
					"hello":             "world",
					"user.name":         "Alice",
					"user.age":          "30",
					"user.address.city": "Wonderland",
					"user.address.zip":  "12345",
					"items.0.name":      "item1",
					"items.0.price":     "10",
					"items.1.name":      "item2",
					"items.1.price":     "20",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FlattenJSON(tt.args.data, tt.args.prefix, tt.args.result)
		})
	}
}
