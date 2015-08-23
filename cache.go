// On disk caching system, intended to be used for compilation data.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"
)

// A cache interface
type Cache interface {
	// Store data in the cache
	store(key []byte, data []byte) error

	// Load data from the cache which matches the key
	load(key []byte) ([]byte, error)

	// TODO we want some stats about hit and miss rate
}

type HashKey [md5.Size]byte

// Represents the state of a file
type fileData struct {
	usetime time.Time // Time of last use for this file
	path    string    // Path to the file on disk
	size    int64     // Size of the data on disk
}

// Basic file based data cache
type fileCache struct {
	dir     string               // The directory we are storing data in
	data    map[HashKey]fileData // Maps the key to file disk
	maxsize int64                // Maximum size in bytes of the cache
}

// Create a file based cache from the given directory, the format for the files
func NewfileCache(directory string, maxsize int64) (*fileCache, error) {
	// Create the directory if needed
	err := os.MkdirAll(directory, 0755)

	if err != nil {
		return nil, err
	}

	// Scan the directory for file
	files, err := ioutil.ReadDir(directory)

	if err != nil {
		return nil, err
	}

	// Create the cache
	c := new(fileCache)
	c.dir = directory
	c.data = make(map[HashKey]fileData)
	c.maxsize = maxsize

	// Load the data into the cache
	for _, file := range files {
		// Check to make sure it's the correct length
		var h HashKey
		n := file.Name()

		if len(n) != len(h) {
			log.Print("Odd file name:", n, "in cache:", directory)
			continue
		}

		// Convert string to hash
		h, err = stringToHash(n)

		if err == nil {
			// Save entry in our cache
			c.data[h] = fileData{
				usetime: file.ModTime(),
				path:    directory + "/" + n,
				size:    file.Size(),
			}
		} else {
			// Ignore this guy
			log.Print("Failed to parse file name:", n, "in cache:", directory)
		}
	}

	// Trim it down to size
	err = c.trim()

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Stores the given file on disk, trims old files if we are too big
func (c *fileCache) store(key []byte, data []byte) error {
	// Hash the key data
	h := c.hash(key)

	// Build the fileData
	fd := fileData{
		usetime: time.Now(),
		path:    c.hashToPath(h),
		size:    int64(len(data)),
	}

	// Store the data to disk
	err := ioutil.WriteFile(fd.path, data, 0644)

	if err != nil {
		return err
	}

	// Update the in memory hash
	c.data[h] = fd

	// Trim the data
	err = c.trim()

	return err
}

// Load the data from disk corresponding to the key if it's there, otherwise
// error
func (c *fileCache) load(key []byte) (d []byte, err error) {
	// Hash the key data
	h := c.hash(key)

	// Check the in memory hash
	if fd, ok := c.data[h]; ok {
		// We have the data so load if from disk
		d, err = ioutil.ReadFile(fd.path)

		return
	} else {
		// No data just return and error
		err = fmt.Errorf("Data not in cache")
		return
	}
}

// This is really in-efficient trim function but it's a good first pass
func (c *fileCache) trim() error {
	// While still too big
	for {
		// Compute the total size and find the oldest file
		var total int64
		cur := time.Now()
		var oldest HashKey

		for k, fd := range c.data {
			total += fd.size

			if fd.usetime.Before(cur) {
				oldest = k
			}
		}

		if total < c.maxsize {
			return nil
		}

		// Remove the map
		fd := c.data[oldest]

		delete(c.data, oldest)

		// Delete from disk
		err := os.Remove(fd.path)

		if err != nil {
			return err
		}
	}

	return nil
}

// Compute the hash from the given key data
func (c *fileCache) hash(key []byte) HashKey {
	return md5.Sum(key)
}

// Compute the hash from the given key data
func (c *fileCache) hashToPath(h HashKey) string {
	return c.dir + "/" + hashToString(h)
}

func hashToString(h HashKey) string {
	return fmt.Sprintf("%x", h)
}

func stringToHash(n string) (HashKey, error) {
	// Attempt a decode
	b, err := hex.DecodeString(n)

	var h HashKey

	// Bail out on error
	if err != nil {
		return h, err
	}

	// Convert the rest
	copy(h[:], b)

	return h, nil
}
