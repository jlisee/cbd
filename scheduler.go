// This defines a scheduler interface.  The interface schedules jobs among it's
// the free workers.  It only matches machines which have IP addresses that
// indicate they can talk to one another.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
)

// The information needed
type SchedulerRequest struct {
	// Where the result is sent
	r chan WorkerResponse

	// Addresses of the client
	addrs []net.IPNet
}

func NewSchedulerRequest(addrs []net.IPNet) *SchedulerRequest {
	req := new(SchedulerRequest)
	req.r = make(chan WorkerResponse, 1)
	req.addrs = addrs

	return req
}

// Schedules jobs amongst a pool of workers
type Scheduler interface {
	// Put in a request to schedule a job
	schedule(r *SchedulerRequest) error

	// Mark job completed
	completed(cj CompletedJob) error

	// Add resource
	addWorker(state WorkerState) error

	// Update resource
	updateWorker(state WorkerState) error

	// Remove resource
	removeWorker(worker string) error

	// Get current work state
	getWorkerState() WorkerStateList

	/// TODO: figure out a way to remove me, this is just a test function
	findWorker(addrs []net.IPNet) (string, net.IPNet, int, error)

	// TODO: something to dump current queue information

	// TODO: we need to be able to cancel requests, create a simple GUID generator
	// http://play.golang.org/p/7JJDx4GL77
}

type FifoScheduler struct {
	workers map[string]WorkerState // All the currently active workers
	smutex  *sync.Mutex            // Protects access to all state

	// TODO: consider container/list which would have less copying
	requests []*SchedulerRequest // Waiting requests
}

func newFifoScheduler() *FifoScheduler {
	s := new(FifoScheduler)
	s.workers = make(map[string]WorkerState)
	s.smutex = new(sync.Mutex)
	s.requests = make([]*SchedulerRequest, 0, 100)

	return s
}

func (s *FifoScheduler) schedule(req *SchedulerRequest) error {
	// Determine if we have something available right now
	s.smutex.Lock()
	defer s.smutex.Unlock()

	// If there are no workers bail out early
	if len(s.workers) == 0 {
		req.r <- WorkerResponse{Type: NoWorkers}
		return nil
	}

	/// TODO: handle no source address check explicitly at this level
	host, addr, port, err := findFreeWorker(&s.workers, req.addrs)

	if err == nil {
		// Build valid response
		wr := WorkerResponse{
			Type:    Valid,
			Host:    host,
			Address: addr,
			Port:    port,
		}

		// Write it to channel
		req.r <- wr

	} else {
		// We did not find a worker so queue it
		s.requests = append(s.requests, req)

		// Then tell the waiting user they are queue
		// TODO: should we do this, or just use the absence?
		req.r <- WorkerResponse{Type: Queued}
	}

	return nil
}

func (s *FifoScheduler) completed(cj CompletedJob) error {
	s.smutex.Lock()
	defer s.smutex.Unlock()

	return updateWorkerStats(&s.workers, cj)
}

func (s *FifoScheduler) addWorker(state WorkerState) error {
	s.smutex.Lock()
	defer s.smutex.Unlock()

	s.workers[state.Host] = state

	// Try and schedule queued requests
	s.scheduleRequests()

	return nil
}

func (s *FifoScheduler) updateWorker(update WorkerState) error {
	s.smutex.Lock()
	defer s.smutex.Unlock()

	mergeWorkerState(&s.workers, update)

	// Try and schedule queued requests
	s.scheduleRequests()

	return nil
}

func (s *FifoScheduler) removeWorker(worker string) error {
	s.smutex.Lock()
	defer s.smutex.Unlock()

	delete(s.workers, worker)

	return nil
}

func (s *FifoScheduler) getWorkerState() WorkerStateList {
	s.smutex.Lock()
	defer s.smutex.Unlock()

	// Copy all state into our list
	// TODO: maintain single a list that is just updated instead
	var l WorkerStateList

	for _, state := range s.workers {
		l.Workers = append(l.Workers, state)
	}

	return l
}

/// TODO: remove me just an internal test function
func (s *FifoScheduler) findWorker(addrs []net.IPNet) (string, net.IPNet, int, error) {
	s.smutex.Lock()
	defer s.smutex.Unlock()

	return findFreeWorker(&s.workers, addrs)
}

// Attempts to schedule a request if possible, assumes things are locked
func (s *FifoScheduler) scheduleRequests() {
	// Keep schedule requests until we fail to find a free worker
	for len(s.requests) > 0 {
		// Loop over all requests attempt to find a free worker that
		// matches.
		found := -1

		for idx, req := range s.requests {
			host, addr, port, err := findFreeWorker(&s.workers, req.addrs)

			if err == nil {
				// Build valid response
				wr := WorkerResponse{
					Type:    Valid,
					Host:    host,
					Address: addr,
					Port:    port,
				}

				// Write it to channel
				req.r <- wr

				// Found it!
				found = idx

				break
			}
		}

		// Handle results
		if found >= 0 {
			// We fufilled a request so remove it from our list
			s.requests = append(s.requests[:found], s.requests[found+1:]...)
		} else {
			// We try all requests but couldn't schedule anyone so
			// break out of the loop
			break
		}
	}
}

// Integrate new worker state into existing state map
func mergeWorkerState(workers *map[string]WorkerState, update WorkerState) {
	// Keep the current speed if we already have an entry for this host
	if val, ok := (*workers)[update.Host]; ok {
		speed := val.Speed
		update.Speed = speed
	}

	(*workers)[update.Host] = update
}

// Updates the workers current speed estimate based on the job results, this
// uses New = Old * 0.9 + Update * 0.1 to try and smooth out spikes caused by
// variability.
func updateWorkerStats(workers *map[string]WorkerState, cj CompletedJob) error {
	// Blend in the speed slowly if we already have a speed
	state, ok := (*workers)[cj.Worker]

	if !ok {
		return fmt.Errorf("Could not find worker: %s", cj.Worker)
	}

	if state.Speed == 0 {
		state.Speed = cj.CompileSpeed
	} else {
		state.Speed = state.Speed*0.9 + cj.CompileSpeed*0.1
	}

	(*workers)[cj.Worker] = state

	return nil
}

// findWorker finds a free worker which can connect to any of the given
// addresses and return the corresponding address and port
func findFreeWorker(workers *map[string]WorkerState, addrs []net.IPNet) (string, net.IPNet, int, error) {
	// Error out if we aren't given any addresses to match against
	var empty net.IPNet
	if len(addrs) == 0 {
		return "", empty, 0, errors.New("No source addresses given")
	}

	// Sort the worker IPs so will match local networks before global
	sort.Sort(ByPrivateIPAddr(addrs))

	// Found workers
	var worker WorkerState
	var addr net.IPNet

	worker.Speed = -1
	found := false

	// For now just a simple linear search returning the first free
	for _, wstate := range *workers {
		space := wstate.Capacity - wstate.Load

		if space > 0 {
			// Get a worker IP address that can connect to the client
			maddr, err := getMatchingIP(addrs, wstate.Addrs)

			// Use this worker if it's faster than the last
			if err == nil && worker.Speed < wstate.Speed {
				worker = wstate
				addr = maddr
				found = true
			}
		}
	}

	// Return the fastest found worker
	if found {
		return worker.Host, addr, worker.Port, nil
	}

	return "", empty, 0, errors.New("No available & reachable host")
}
