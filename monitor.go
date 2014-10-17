// Functions and structures relating to the monitoring the state of the cluster
// This is a basic observer pattern implementation.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"time"
)

// CompletedJob is one updated about a job completed on the cluster
type CompletedJob struct {
	Client       string        // Machine that requested the job
	Worker       string        // Worker that build the job
	InputSize    int           // Bytes of source code compiled
	OutputSize   int           // Bytes of object code produced
	CompileTime  time.Duration // How long the job took to complete
	CompileSpeed float64       // Speed rating used for the job
}

// We define the compile speed of a job based
func (c *CompletedJob) computeCompileSpeed() {
	c.CompileSpeed = float64(c.OutputSize) / c.CompileTime.Seconds() / 1024
}
