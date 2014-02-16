package cbuildd

import (
	"testing"
)

func StrsEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestParseArgs(t *testing.T) {
	args := []string{"-c", "data/main.c", "-o", "main.o"}

	b := ParseArgs(args)

	// Make sure the args match
	if !StrsEquals(args, b.Args) {
		t.Errorf("Args are wrong")
	}

	// Make sure we parsed the output properly
	if "main.o" != b.Output {
		t.Errorf("Output path wrong")
	}

	if 3 != b.Oindex {
		t.Errorf("Output index wrong")
	}

	// Now lets do the input
	if 1 != b.Iindex {
		t.Errorf("Input index wrong")
	}

	if "data/main.c" != b.Args[b.Iindex] {
		t.Errorf("Input path wrong")
	}

	// Now lets test the link command is properly recognized
	if b.LinkCommand {
		t.Errorf("Should not be b a link command")
	}
}
