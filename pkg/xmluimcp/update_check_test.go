package xmluimcp

import "testing"

func TestSemverLessThan(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "patch", a: "v0.0.9", b: "v0.0.10", want: true},
		{name: "equal", a: "v0.0.10", b: "0.0.10", want: false},
		{name: "greater", a: "0.1.0", b: "0.0.10", want: false},
		{name: "missing patch", a: "0.1", b: "0.1.1", want: true},
		{name: "prerelease suffix", a: "0.1.0-beta.1", b: "0.1.0", want: false},
		{name: "build suffix", a: "0.1.0+build.1", b: "0.1.1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := semverLessThan(tt.a, tt.b); got != tt.want {
				t.Fatalf("semverLessThan(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
