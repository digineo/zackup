package config

import (
	"testing"
)

// sliceEqual checks if two string slices are identical.
func sliceEqual(a, b []string) bool {
	if (a == nil) != (b == nil) {
		// either is nil, but the other isn't
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

func TestBlacklist(t *testing.T) {
	bla := blacklistArg{long: "--long", short: "-s"}
	mustMatch := func(arg string, expected bool) {
		matches, _ := bla.Matches(arg)
		if expected && !matches {
			t.Errorf("expected to match %q, but didn't\n", arg)
		}
		if !expected && matches {
			t.Errorf("expected to not match %q, but did\n", arg)
		}
	}

	// these hold true, independent of bla.opt
	simpleCases := func() {
		mustMatch("--long", true)
		mustMatch("-s", true)
		mustMatch("--elongated", false)
		mustMatch("---long", false)
		mustMatch("-- long", false)
		mustMatch("--long ", false)
		mustMatch("-s", true)
		mustMatch("-S", false)
	}

	// without opt arg
	simpleCases()
	mustMatch("--long=foo", false)
	mustMatch("--long=", false)
	mustMatch("-s=42", false) // normal form: `-s 42`
	mustMatch("-sasd", false) // normal form: `-s asd`

	// with opt arg
	bla.opt = true
	simpleCases()
	mustMatch("--long=foo", true)
	mustMatch("--long=", true) // XXX: should this fail? is this supported by GNU OptParse?
	mustMatch("-s=42", true)
	mustMatch("-sasd", true)
}

func TestRsyncArgs(t *testing.T) {
	c := &RsyncConfig{
		Arguments: []string{
			"--perms",
			"--daemon", // simple match
			"--recursive",
			"-e", "ssh -oSomething=yes", // -e swallows next token
			"--include", "a", // --include swallows next token
			"--bwrate", "5000",
			"--include=b",             // --include=* does NOT swallow next token
			"--rsh=ssh -oThird=turd",  // similar to --include=*
			"--rsh", "ssh -oOther=no", // similar to -e
			"--numeric-ids",
		},
	}

	// this tests the following:
	//
	// 1. are blacklisted flags removed?
	// 2. if a blacklisted flag swallows the next token, is that removed as well?
	//    - does this hold true only for the form `--long tok`, but not for `--long=tok`?
	// 3. is the order kept?

	expected := []string{"--perms", "--recursive", "--bwrate", "5000", "--numeric-ids"}
	actual := c.args()

	if !sliceEqual(actual, expected) {
		t.Errorf("actual=%q, expected=%q\n", actual, expected)
	}
}
