package api

import "testing"

func TestValidReactions(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{{[]string{"❤️", "🔥"}, true}, {nil, false}, {[]string{"🔥", "🔥"}, false}, {[]string{""}, false}}
	for _, tc := range cases {
		if got := validReactions(tc.in); got != tc.want {
			t.Fatalf("validReactions(%v)=%v", tc.in, got)
		}
	}
}
