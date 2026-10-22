package gh

import "testing"

func TestNewest(t *testing.T) {
	data := []byte(`[
		{"number": 1, "state": "CLOSED", "updatedAt": "2026-01-01T00:00:00Z"},
		{"number": 3, "state": "MERGED", "updatedAt": "2026-03-01T00:00:00Z"},
		{"number": 2, "state": "OPEN",   "updatedAt": "2026-02-01T00:00:00Z"}
	]`)
	pr := newest(data)
	if pr == nil || pr.Number != 3 || pr.State != "MERGED" {
		t.Errorf("newest() = %+v, want #3 MERGED", pr)
	}
}

func TestNewestEmpty(t *testing.T) {
	for _, in := range []string{`[]`, ``, `not json`} {
		if pr := newest([]byte(in)); pr != nil {
			t.Errorf("newest(%q) = %+v, want nil", in, pr)
		}
	}
}
