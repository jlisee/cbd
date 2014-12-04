package cbd

import (
	//	"bytes"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"
)

type WorkerTestCase struct {
	update    WorkerState    // Update to apply
	completed []CompletedJob // Compile jobs (applied after update)
	host      string
	port      int
	empty     bool
	error     bool        // True if we expect and error
	addrs     []net.IPNet // Client IPs
}

// A channel based deadline reader writer
type ChannelReadWriter struct {
	bytes chan byte // Channels bytes come in and out on
	cl    chan bool // Used to signal the channel closed
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

	cr.bytes = make(chan byte)
	cr.cl = make(chan bool)

	return cr
}

// Read data until there is nothing in the channel
func (s *ChannelReadWriter) Read(p []byte) (n int, err error) {
	n = 0
	err = nil

	for n = 0; n < len(p); {
		done := false

		select {
		case b := <-s.bytes:
			p[n] = b
			n++
		case _ = <-s.cl:
			err = fmt.Errorf("Channel closed")
		default:
			done = true
		}

		if done {
			break
		}
	}

	return n, err
}

// Write each byte into the channel
func (s *ChannelReadWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		s.bytes <- b
	}

	return len(p), nil
}

func (s *ChannelReadWriter) Close() {
	s.cl <- true
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

	updates := []WorkerTestCase{
		// First we make sure that the empty case works
		WorkerTestCase{
			empty:     true,
			error:     true,
			completed: make([]CompletedJob, 0, 0),
			addrs:     make([]net.IPNet, 0, 0),
		},
		WorkerTestCase{
			update: WorkerState{
				Host: "smith",
				Addrs: []net.IPNet{
					{net.IPv4(192, 1, 1, 1), net.IPv4Mask(255, 255, 255, 0)},
				},
				Port:     56,
				Capacity: 5,
				Load:     2,
			},
			completed: make([]CompletedJob, 0, 0),
			host:      "smith",
			port:      56,
			empty:     false,
			error:     false,
			addrs: []net.IPNet{
				{net.IPv4(192, 1, 1, 2), net.IPv4Mask(255, 255, 255, 0)},
			},
		},
		WorkerTestCase{
			update: WorkerState{
				Host: "speedy",
				Addrs: []net.IPNet{
					{net.IPv4(192, 1, 1, 3), net.IPv4Mask(255, 255, 255, 0)},
				},
				Port:     56,
				Capacity: 2,
				Load:     0,
			},
			completed: []CompletedJob{
				{Worker: "speedy", CompileSpeed: 5},
				{Worker: "smith", CompileSpeed: 1},
			},
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
			s.updateStats(cj)
		}

		// TODO: don't ignore address
		host, _, port, err := s.findWorker(u.addrs)

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

		if host != u.host {
			t.Error("Wrong host expected:", u.host, "found", host)
		}

		if port != u.port {
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

		if host != r.Host {
			t.Errorf("Got host: \"%s\" wanted: %s", r.Host, host)
		}

		if port != r.Port {
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
	var host string

	for {
		var err error

		host, _, _, err = s.findWorker(clientAddrs)

		if err == nil {
			break
		}
	}

	// Make sure we have the right host back
	if host != "smith" {
		t.Error("Bad worker")
	}

	// Now lets close the connection which should drop the worker
	conn.Close()

	// Now lets wait for that drop
	for {
		var err error

		_, _, _, err = s.findWorker(clientAddrs)

		if err != nil {
			break
		}
	}
}

// TODO: test the compile speed update here

// TODO: we should figure out how to test monitoring here
