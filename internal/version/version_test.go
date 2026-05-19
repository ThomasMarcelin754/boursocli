package version

import "testing"

func TestVersion(t *testing.T) {
	if String() == "" {
		t.Fatal("empty version string")
	}
	i := Info()
	for _, k := range []string{"version", "commit", "date"} {
		if i[k] == "" {
			t.Fatalf("Info missing %q", k)
		}
	}
}
