// Functions and structures relating to the monitoring the state of the cluster
// This is a basic observer pattern implementation.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

// observerDst represents a location to send updates to
type observerDst struct {
	host string
	ch   chan interface{}
}

type updatePublisher struct {
	updates     chan interface{} // Completed jobs
	newMonitor  chan observerDst // Channel to send new monitors
	stopMonitor chan string      // Channel used to stop a monitor
}

func newUpdatePublisher() *updatePublisher {
	p := new(updatePublisher)
	p.updates = make(chan interface{})
	p.newMonitor = make(chan observerDst)
	p.stopMonitor = make(chan string)

	go p.handlePublish()

	return p
}

func (p *updatePublisher) addObs(h string, c chan interface{}) {
	p.newMonitor <- observerDst{host: h, ch: c}
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
