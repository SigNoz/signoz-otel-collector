package keycheck

import "testing"

func Test_IsRandomKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "Valid UUID",
			key:  "123e4567-e89b-12d3-a456-426614174000",
			want: true,
		},
		{
			name: "Valid Hex String",
			key:  "abcdef1234567890abcdef1234567890",
			want: true,
		},
		{
			name: "Valid Base64 String",
			key:  "QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			want: true,
		},
		{
			name: "Long String Without Vowels",
			key:  "bcdfghjklmnpqrstvwxyz",
			want: false,
		},
		{
			name: "Mixed Case with Digits",
			key:  "Abcdef123456",
			want: false,
		},
		{
			name: "Short String",
			key:  "short",
			want: false,
		},
		{
			name: "String with Vowels",
			key:  "aeiouAEIOU",
			want: false,
		},
		{
			name: "Mixed Case without Digits",
			key:  "Abcdefghijklmnop",
			want: false,
		},
		{
			name: "Empty String",
			key:  "",
			want: false,
		},
		{
			name: "String with Special Characters",
			key:  "abc!@#123",
			want: false,
		},
		{
			name: "String with Only Digits",
			key:  "1234567890123456",
			want: true,
		},
		{
			name: "Flattened JSON Key with UUID",
			key:  "x.y.123e4567-e89b-12d3-a456-426614174000",
			want: true,
		},
		{
			name: "Flattened JSON Key with Hex",
			key:  "x.y.abcdef1234567890abcdef1234567890",
			want: true,
		},
		{
			name: "Flattened JSON Key with Base64",
			key:  "x.y.QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			want: true,
		},
		{
			name: "Flattened JSON Key with Regular String",
			key:  "x.y.orderId",
			want: false,
		},
		{
			name: "Deeply Nested JSON Path with ID",
			key:  "user.profile.settings.preferences.id.123456789",
			want: false,
		},
		{
			name: "Multiple UUIDs in Path",
			key:  "parent.123e4567-e89b-12d3-a456-426614174000.child.987fcdeb-51a2-43d7-9876-543210987654",
			want: true,
		},
		{
			name: "Mixed Random and Meaningful Keys",
			key:  "user.123e4567-e89b-12d3-a456-426614174000.name",
			want: true,
		},
		{
			name: "Long Path with Multiple Random Segments",
			key:  "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z",
			want: false,
		},
		{
			name: "Path with Base64 in Middle",
			key:  "user.data.QWxhZGRpbjpvcGVuIHNlc2FtZQ==.profile",
			want: true,
		},
		{
			name: "Path with Hex in Middle",
			key:  "config.abcdef1234567890abcdef1234567890.settings",
			want: true,
		},
		{
			name: "Path with Multiple Special Characters",
			key:  "user@domain.com.profile.settings",
			want: false,
		},
		{
			name: "Path with Timestamp-like Numbers",
			key:  "events.1647123456789.data",
			want: true,
		},
		{
			name: "Path with Multiple Dots and Numbers",
			key:  "v1.2.3.4.5.6.7.8.9.0",
			want: false,
		},
		{
			name: "Path With orderID",
			key:  "methodArgs.txn_map.KWIK3J3DCFYS3584069MX",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRandomKey(tt.key); got != tt.want {
				t.Errorf("isRandomKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_IsCardinal(t *testing.T) {
	tests := []struct {
		key        string
		isCardinal bool
	}{
		{
			key:        "value-test",
			isCardinal: false,
		},
		{
			key:        "value@test",
			isCardinal: false,
		},
		{
			key:        "value:test",
			isCardinal: false,
		},
		{
			key:        "value_test",
			isCardinal: false,
		},
		{
			key:        "value1",
			isCardinal: true,
		},
		{
			key:        "value`",
			isCardinal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsCardinal(tt.key); got != tt.isCardinal {
				t.Errorf("IsCardinal() = %v, want %v", got, tt.isCardinal)
			}
		})
	}
}
