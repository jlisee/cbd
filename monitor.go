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

// monitorDst represents a location to send job completions to
type monitorDst struct {
	host string
	ch   chan interface{}
}

type updatePublisher struct {
	updates     chan interface{} // Completed jobs
	newMonitor  chan monitorDst  // Channel to send new monitors
	stopMonitor chan string      // Channel used to stop a monitor
}

func newUpdatePublisher() *updatePublisher {
	p := new(updatePublisher)
	p.updates = make(chan interface{})
	p.newMonitor = make(chan monitorDst)
	p.stopMonitor = make(chan string)

	go p.handlePublish()

	return p
}

func (p *updatePublisher) addObs(h string, c chan interface{}) {
	p.newMonitor <- monitorDst{host: h, ch: c}
}

func (p *updatePublisher) removeObs(h string) {
	p.stopMonitor <- h
}

func (p *updatePublisher) publish(j interface{}) {
	p.updates <- j
}

func (p *updatePublisher) handlePublish() {
	obs := make(map[string]chan interface{})
	more := true

	for more {
		// Read in new jobs and update observers map as needed
		var cj interface{}

		select {
		case mDst := <-p.newMonitor:
			// Add new destination
			obs[mDst.host] = mDst.ch
		case h := <-p.stopMonitor:
			// Remove destination
			delete(obs, h)

		case cj, more = <-p.updates:
			// Send to all channels that are ready
			for _, dst := range obs {
				select {
				case dst <- cj:
					// Message away
				default:
					// Nothing to do
				}
			}
		}
	}
}
