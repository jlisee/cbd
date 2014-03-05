// This file contains utility functions that are miscellaneous or fill in
// gaps in Go's standard library.  Either way that are not specific to this
// project.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// The result of running a command
type ExecResult struct {
	Output []byte // Output of the command
	Return int    // Return code of program
}

// Executes, returning the stdout if the program fails (the return code is
// ignored)
func RunCmd(prog string, args []string) (result ExecResult, err error) {
	// fmt.Printf("Run: %s ", prog)
	// for _, arg := range args {
	// 	fmt.Printf("%s ", arg)
	// }
	// fmt.Println()

	cmd := exec.Command(prog, args...)

	// Setup the buffer to hold the output
	// TODO: consider caching this buffer
	buffer := new(bytes.Buffer)

	cmd.Stdout = buffer
	cmd.Stderr = buffer

	err = cmd.Run()

	// Copy over our buffer
	result.Output = buffer.Bytes()

	// Get the return code out of the error
	if err != nil {
		result.Return = -1

		// Possibly Linux specific example
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				result.Return = status.ExitStatus()
			}
		}
	}

	return
}

// copies dst to src location, no metadata is copied
func Copyfile(dst, src string) error {
	s, err := os.Open(src)

	if err != nil {
		return err
	}

	// No need to check errors on read only file, we already got everything
	// we need from the filesystem, so nothing can go wrong now.
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(d, s); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}

// Generates and opens a temporary file with a defined prefix and suffix
// This is the same api as ioutil.TempFile accept it accepts a suffix
//  TODO: see if this is too slow
func TempFile(dir, prefix string, suffix string) (f *os.File, err error) {
	if dir == "" {
		dir = os.TempDir()
	}

	// The maximum size of random file count
	// TODO: see if we can do this at package scope somehow
	var maxRand *big.Int = big.NewInt(0)
	maxRand.SetString("FFFFFFFFFFFFFFFF", 16)

	var randNum *big.Int

	for i := 0; i < 10000; i++ {
		// Generate random part of the path name
		randNum, err = rand.Int(rand.Reader, maxRand)

		if err != nil {
			return
		}

		// Transform to an int
		randString := hex.EncodeToString(randNum.Bytes())

		// Attempt to open file and fail if it already exists
		name := filepath.Join(dir, prefix+randString+suffix)
		f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		break
	}
	return
}

// GetLoadAverage returns the 1 minute system load average
// TODO: linux only, support more of unix by using cgo and getloadavg
func GetLoadAverage() (float64, error) {
	d, err := ioutil.ReadFile("/proc/loadavg")

	if err != nil {
		return 0.0, err
	}

	parts := strings.Split(string(d), " ")

	load, err := strconv.ParseFloat(parts[0], 64)

	if err != nil {
		return 0.0, err
	}

	return load, nil
}
