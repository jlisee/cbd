// Tests for server functionality.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"bytes"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"
)

type WorkerTestCase struct {
	// Input data
	update    WorkerState    // Update to apply
	completed []CompletedJob // Compile jobs (applied after update)

	// Control flags
	empty bool // Means there is no worker state
	error bool // True if we expect and error

	// Successful results
	id    MachineID
	host  string
	port  int
	addrs []net.IPNet // Client IPs
}

// A channel based deadline reader writer
type ChannelReadWriter struct {
	bytesIn  chan byte // Write writes to this channel
	bytesOut chan byte // Read read from this channel
	rCl      chan bool // Used to stop the Read function
	wCl      chan bool // Used to stop the Write function
	shCl     chan bool // Used to stop the shuffle goroutine
	open     bool      // Channel is open
}

func newChannelReadWriter() *ChannelReadWriter {
	// This is a huge back, but we need to set the this to at least more than
	// two so our channel based IO Reader/Writer works. This is because we have
	// the reader and the writer directly access the same channel. (putting a
	// buffer in their would probably be better)
	maxProcs := runtime.GOMAXPROCS(0)
	if maxProcs < 2 {
		runtime.GOMAXPROCS(2)
	}

	cr := new(ChannelReadWriter)

	cr.bytesIn = make(chan byte)
	cr.bytesOut = make(chan byte)
	cr.rCl = make(chan bool, 1)
	cr.wCl = make(chan bool, 1)
	cr.shCl = make(chan bool, 1)
	cr.open = true

	// Moves data between bytesIn and bytesOut
	go cr.shuffle()

	return cr
}

// Moves data between are channels
func (s *ChannelReadWriter) shuffle() {
	var buf bytes.Buffer
	d := make([]byte, 1)
	// Tracks whether we have a byte in our inFlight buffer
	inFlight := false

	run := true

	for run {
		// If we have data to send keep sending until we are done
		for run && (buf.Len() > 0 || inFlight) {
			// Make sure we have a byte to send
			if !inFlight {
				buf.Read(d)
				inFlight = true
			}

			// Wait for a chance to send, data to read, or a stop signal
			select {
			case s.bytesOut <- d[0]:
				// data sent
				inFlight = false
			case b := <-s.bytesIn:
				// keep pulling in more data
				buf.Write([]byte{b})
			case _ = <-s.shCl:
				run = false
			}
		}

		// No data left to send, block until we get more data or stop signal
	Loop:
		for run {
			select {
			case b := <-s.bytesIn:
				buf.Write([]byte{b})
				break Loop
			case _ = <-s.shCl:
				run = false
			}
		}
	}
}

// Read data until there is nothing in the channel
func (s *ChannelReadWriter) Read(p []byte) (n int, err error) {
	n = 0
	err = nil

	if s.open {
		// Timeout if we don't get any data, this is ugly but the higher level
		// API's don't like this return to spin returning n == 0, err == nil,
		// while we wait for data to come in
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(250 * time.Millisecond)
			timeout <- true
		}()

	Loop:
		for n = 0; n < len(p); {
			select {
			case b := <-s.bytesOut:
				p[n] = b
				n++
			case _ = <-s.rCl:
				err = fmt.Errorf("Channel closed")
				break Loop
			case <-timeout:
				// No data
				break Loop
			}
		}
	}

	return n, err
}

// Write each byte into the channel
func (s *ChannelReadWriter) Write(p []byte) (n int, err error) {
	n = 0
	err = nil

	if s.open {
	Loop:
		for _, b := range p {
			select {
			case s.bytesIn <- b:
				n += 1
			case _ = <-s.rCl:
				err = fmt.Errorf("Channel closed")
				break Loop
			}
		}
	}

	return n, err
}

func (s *ChannelReadWriter) Close() {
	s.open = false
	s.rCl <- true
	s.wCl <- true
	s.shCl <- true
}

func (s *ChannelReadWriter) SetReadDeadline(t time.Time) error {
	// Do nothing
	return nil
}

func (s *ChannelReadWriter) SetWriteDeadline(t time.Time) error {
	// Do nothing
	return nil
}

// Our tests
func TestServerWorkerTracking(t *testing.T) {
	s := NewServerState()

	smith_id := MachineID("11:23:45:67:89:ab")
	speedy_id := MachineID("01:23:45:67:89:ab")

	updates := []WorkerTestCase{
		// First we make sure that the empty case works
		WorkerTestCase{
			empty:     true,
			error:     true,
			completed: make([]CompletedJob, 0, 0),
			addrs:     make([]net.IPNet, 0, 0),
		},

		// Then put in a worker with available space and a request for him
		WorkerTestCase{
			update: WorkerState{
				ID:   smith_id,
				Host: "smith",
				Addrs: []net.IPNet{
					{net.IPv4(192, 1, 1, 1), net.IPv4Mask(255, 255, 255, 0)},
				},
				Port:     56,
				Capacity: 5,
				Load:     2,
			},
			completed: make([]CompletedJob, 0, 0),
			id:        smith_id,
			host:      "smith",
			port:      56,
			empty:     false,
			error:     false,
			addrs: []net.IPNet{
				{net.IPv4(192, 1, 1, 2), net.IPv4Mask(255, 255, 255, 0)},
			},
		},

		// Then we put in another worker along with an update that marks him speedy
		// and we make sure he gets the new job
		WorkerTestCase{
			update: WorkerState{
				ID:   speedy_id,
				Host: "speedy",
				Addrs: []net.IPNet{
					{net.IPv4(192, 1, 1, 3), net.IPv4Mask(255, 255, 255, 0)},
				},
				Port:     56,
				Capacity: 2,
				Load:     0,
			},
			completed: []CompletedJob{
				{Worker: MachineName{ID: speedy_id, Host: "speedy"}, CompileSpeed: 5},
				{Worker: MachineName{ID: smith_id, Host: "smith"}, CompileSpeed: 1},
			},
			id:   speedy_id,
			host: "speedy",
			port: 56,
			addrs: []net.IPNet{
				{net.IPv4(192, 1, 1, 2), net.IPv4Mask(255, 255, 255, 0)},
			},
		},
	}

	for _, u := range updates {
		if !u.empty {
			s.updateWorker(u.update)
		}

		// Update stats based on our completed jobs
		for _, cj := range u.completed {
			err := s.updateStats(cj)

			if err != nil {
				t.Error("Failed to update stats: ", err)
			}
		}

		// TODO: don't ignore address
		wr, err := s.sch.findWorker(u.addrs)

		// Test one where expect nothing back
		if u.error {
			if err == nil {
				t.Error("Expected error")
			}
			continue
		}

		// Continue with normal testing
		if err != nil {
			t.Error("Find worker error: ", err)
		}

		if wr.ID != u.id {
			t.Error("Wrong ID expected:", wr.ID, "found", u.id)
		}

		if wr.Host != u.host {
			t.Error("Wrong host expected:", u.host, "found", wr.Host)
		}

		if wr.Port != u.port {
			t.Error("Wrong port")
		}

		// Now lets make sure the comms function works
		var network MockConn
		mc := NewMessageConn(&network, time.Duration(10)*time.Second)

		// TODO: have to set this IP address carefully
		var req WorkerRequest
		req.Addrs = u.addrs
		err = s.processWorkerRequest(mc, req)

		if err != nil {
			t.Error("Process Error: ", err)
			continue
		}

		r, err := mc.ReadWorkerResponse()

		if err != nil {
			t.Error("Read Error: ", err)
			continue
		}

		if u.id != r.ID {
			t.Errorf("Got id \"%s\" wanted: \"%s\"", r.ID, u.id)
		}

		if u.host != r.Host {
			t.Errorf("Got host: \"%s\" wanted: \"%s\"", r.Host, u.host)
		}

		if u.port != r.Port {
			t.Error("Wrong port")
		}
	}

}

// Make sure we drop a worker after a connection is severed
func TestWorkerDrop(t *testing.T) {
	// Start up server listening on our channel based connection
	s := NewServerState()

	conn := newChannelReadWriter()
	mc := NewMessageConn(conn, time.Duration(10)*time.Second)

	go s.handleConnection(mc)

	// Now lets connect a worker
	ws := WorkerState{
		Host: "smith",
		Addrs: []net.IPNet{
			{net.IPv4(192, 1, 1, 1), net.IPv4Mask(255, 255, 255, 0)},
		},
		Port:     56,
		Capacity: 1,
		Load:     0,
	}

	clientAddrs := []net.IPNet{
		{net.IPv4(192, 1, 1, 2), net.IPv4Mask(255, 255, 255, 0)},
	}

	mc.Send(ws)

	// We block until we are able to find a worker, which means our connection
	// was successful
	var wr WorkerResponse

	for {
		var err error

		wr, err = s.sch.findWorker(clientAddrs)

		if err == nil {
			break
		}
	}

	// Make sure we have the right host back
	if wr.Host != "smith" {
		t.Error("Bad worker")
	}

	// Now lets close the connection which should drop the worker
	conn.Close()

	// Now lets wait for that drop
	for {
		var err error

		_, err = s.sch.findWorker(clientAddrs)

		if err != nil {
			break
		}
	}
}

// TODO: test the compile speed update here

// TODO: we should figure out how to test monitoring here

// TODO: need a test for queue worker behavior somehow

func TestWorkerQueue(t *testing.T) {
	// Setup some busy workers

	// Ask for a worker

	// Make sure we get queued response back

	// Free up a worker by finishing a job

	// Make sure we now get back the worker we expect
}
