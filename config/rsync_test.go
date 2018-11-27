package config

import (
	"testing"
)

func sliceEqual(a, b []string) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestRsyncArgs(t *testing.T) {
	c := &RsyncConfig{
		Arguments: []string{
			"--perms",
			"--daemon",
			"--recursive",
			"-e", "ssh -oSomething=yes",
			"--include", "a",
			"--bwrate", "5000",
			"--include=b",
			"--rsh", "ssh -oOther=no",
			"--rsh=ssh -oThird=turd",
			"--numeric-ids",
		},
	}

	expected := []string{"--perms", "--recursive", "--bwrate", "5000", "--numeric-ids"}
	actual := c.args()

	if !sliceEqual(actual, expected) {
		t.Errorf("actual=%q, expected=%q\n", actual, expected)
	}
}
