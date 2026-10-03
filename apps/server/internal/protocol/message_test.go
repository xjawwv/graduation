package protocol

import "testing"

func TestValidDisplayBackground(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"transparent", true},
		{"#09090b", true},
		{"#A0B1C2", true},
		{"", false},
		{"#fff", false},
		{"09090b", false},
		{"#09090z", false},
	}
	for _, tc := range cases {
		if got := ValidDisplayBackground(tc.value); got != tc.want {
			t.Fatalf("ValidDisplayBackground(%q)=%v, want %v", tc.value, got, tc.want)
		}
	}
}
