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
}

// monitorDst represents a location to send job completions to
type monitorDst struct {
	host string
	ch   chan CompletedJob
}

type completedJobPublisher struct {
	jobsComplete chan CompletedJob // Completed jobs
	newMonitor   chan monitorDst   // Channel to send new monitors
	stopMonitor  chan string       // Channel used to stop a monitor
}

func newCompletedJobPublisher() *completedJobPublisher {
	p := new(completedJobPublisher)
	p.jobsComplete = make(chan CompletedJob)
	p.newMonitor = make(chan monitorDst)
	p.stopMonitor = make(chan string)

	go p.handlePublish()

	return p
}

func (p *completedJobPublisher) addObs(h string, c chan CompletedJob) {
	p.newMonitor <- monitorDst{host: h, ch: c}
}

func (p *completedJobPublisher) removeObs(h string) {
	p.stopMonitor <- h
}

func (p *completedJobPublisher) publish(j CompletedJob) {
	p.jobsComplete <- j
}

func (p *completedJobPublisher) handlePublish() {
	obs := make(map[string]chan CompletedJob)
	more := true

	for more {
		// Read in new jobs and update observers map as needed
		var cj CompletedJob

		select {
		case mDst := <-p.newMonitor:
			// Add new destination
			obs[mDst.host] = mDst.ch
		case h := <-p.stopMonitor:
			// Remove destination
			delete(obs, h)

		case cj, more = <-p.jobsComplete:
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
