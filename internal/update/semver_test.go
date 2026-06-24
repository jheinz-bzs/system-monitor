package update

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"newer patch", "v1.2.3", "v1.2.2", 1},
		{"older minor", "v1.1.0", "v1.2.0", -1},
		{"equal", "v2.0.0", "v2.0.0", 0},
		{"equal mixed v-prefix", "1.0.0", "v1.0.0", 0},
		{"major dominates", "v2.0.0", "v1.9.9", 1},
		{"prerelease suffix ignored", "v1.2.3-rc1", "v1.2.3", 0},
		{"short form fills zeros", "v1.2", "v1.2.0", 0},
		{"malformed a sorts lowest", "garbage", "v1.0.0", -1},
		{"malformed b sorts lowest", "v1.0.0", "nope", 1},
		{"both malformed equal", "x", "y", 0},
		{"dev never newer", "dev", "v0.0.1", -1},
	}
	for _, tt := range tests {
		if got := compare(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: compare(%q, %q) = %d, want %d", tt.name, tt.a, tt.b, got, tt.want)
		}
	}
}

func TestIsReleaseVersion(t *testing.T) {
	for _, v := range []string{"v1.0.0", "1.2.3", "v0.0.1"} {
		if !isReleaseVersion(v) {
			t.Errorf("isReleaseVersion(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"dev", "", "garbage", "v1.2.3.4", "v1.x.0"} {
		if isReleaseVersion(v) {
			t.Errorf("isReleaseVersion(%q) = true, want false", v)
		}
	}
}
