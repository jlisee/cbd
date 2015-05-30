// Tests for our scheduler.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"net"
	"testing"
)

type SchedulerTestCase struct {
	// Input data
	update    WorkerState    // Update to apply
	completed []CompletedJob // Compile jobs (applied after update)

	// Control flags
	empty bool // Means there is no worker state
	error bool // True if we expect and error

	// Successful results
	host  string
	port  int
	addrs []net.IPNet // Client IPs
}

func TestScheduler(t *testing.T) {
	// Create our scheduler
	var sch Scheduler
	sch = newFifoScheduler()

	// Start by schedule something when we have no workers
	addrs := []net.IPNet{{net.IPv4(192, 1, 1, 3), net.IPv4Mask(255, 255, 255, 0)}}
	sreq := NewSchedulerRequest(addrs)

	err := sch.schedule(sreq)

	if err != nil {
		t.Error("Schedule error: ", err)
	}

	// Make sure we get a no workers response back
	res := <-sreq.r

	if NoWorkers != res.Type {
		t.Error("Response should of been no workers, but got:", res.Type)
	}

	// Ask Try and schedule something when there are no workers at all

	// Setup some busy workers
	foo := WorkerState{
		Host: "foo",
		Addrs: []net.IPNet{
			{net.IPv4(192, 1, 1, 2), net.IPv4Mask(255, 255, 255, 0)},
		},
		Port:     56,
		Capacity: 3,
		Load:     3,
	}

	sch.addWorker(
		WorkerState{
			Host: "bar",
			Addrs: []net.IPNet{
				{net.IPv4(192, 1, 1, 1), net.IPv4Mask(255, 255, 255, 0)},
			},
			Port:     56,
			Capacity: 5,
			Load:     5,
		},
	)

	sch.addWorker(foo)

	// Schedule a new request asking for a worker
	sreq = NewSchedulerRequest(addrs)

	err = sch.schedule(sreq)

	if err != nil {
		t.Error("Schedule error: ", err)
	}

	// Make sure we get queued response back
	res = <-sreq.r

	if Queued != res.Type {
		t.Error("Response should of been queued, but got:", res.Type)
	}

	// Free up a worker by updating it's state
	foo.Load = 0

	sch.updateWorker(foo)

	// Make sure we now get back the worker we expect
	res = <-sreq.r

	if Valid != res.Type {
		t.Error("Response should of been valid, but got:", res.Type)
	}

	if foo.Host != res.Host {
		t.Error("Should of gotten foo host but got:", res.Host)
	}
}
