package httpapi

import "testing"

func TestNormalizeLauncherCategory(t *testing.T) {
	for _, test := range []struct{ input, want string }{{"shooter", "shooter"}, {"STRATEGY", "strategy"}, {"browser", "other"}, {"", "other"}} {
		if got := normalizeLauncherCategory(test.input); got != test.want {
			t.Fatalf("%q: got %q want %q", test.input, got, test.want)
		}
	}
}
