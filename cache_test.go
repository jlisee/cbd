// Tests for our file based caching system
//
// Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"crypto/md5"
	"testing"
)

func TestFileCacheLoad(t *testing.T) {
	// Create a temporary directory and defer cleanup

	// Data setup we want to store things

	// Dump the data on disk

	// Load each make sure we get it back

	// Test stuff that not there
}

func TestFileCacheStore(t *testing.T) {
	// Create a temporary directory and defer cleanup

	// List of load/store actions and expected results

	// Run through actions
}

func TestHashConversion(t *testing.T) {
	data := []byte("These pretzels are making me thirsty.")
	exp := "b0804ec967f48520697662a204f5fe72"

	h := md5.Sum(data)

	// Test to string
	hstr := hashToString(h)

	if hstr != exp {
		t.Error("Expected hash", exp, "got", hstr)
	}

	// Test from string
	hf, err := stringToHash(exp)

	if err != nil {
		t.Error("Error decoding hex string", err)
	}

	if h != hf {
		t.Error("Expected raw hash", h, "got", hf)
	}
}
