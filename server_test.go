package cbd

import (
	//	"bytes"
	"fmt"
	"runtime"
	"testing"
	"time"
)

type WorkerTestCase struct {
	update WorkerState
	host   string
	port   int
	empty  bool
	error  bool
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
			empty: true,
			error: true,
		},
		WorkerTestCase{
			update: WorkerState{
				Host:     "smith",
				Port:     56,
				Capacity: 5,
				Load:     2,
			},
			host:  "smith",
			port:  56,
			empty: false,
			error: false,
		},
	}

	for _, u := range updates {
		if !u.empty {
			s.updateWorker(u.update)
		}

		host, port, err := s.findWorker()

		// Test one where expect nothing back
		if u.error {
			if err == nil {
				t.Error("Expected error")
			}
			return
		}

		// Continue with normal testing
		if err != nil {
			t.Error("Update error: ", err)
			return
		}

		if host != u.host {
			t.Error("Wrong host")
		}

		if port != u.port {
			t.Error("Wrong port")
		}

		// Now lets make sure the comms function works
		var network MockConn
		mc := NewMessageConn(&network, time.Duration(10)*time.Second)

		err = s.processWorkerRequest(mc)

		if err != nil {
			t.Error("Process Error: ", err)
			return
		}

		r, err := mc.ReadWorkerResponse()

		if err != nil {
			t.Error("Read Error: ", err)
			return
		}

		if host != r.Worker {
			t.Errorf("Got host: \"%s\" wanted: %s", r.Worker, host)
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
		Host:     "smith",
		Port:     56,
		Capacity: 1,
		Load:     0,
	}

	mc.Send(ws)

	// We block until we are able to find a worker, which means our connection
	// was successful
	var host string

	for {
		var err error

		host, _, err = s.findWorker()

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

		_, _, err = s.findWorker()

		if err != nil {
			break
		}
	}
}

// TODO: we should figure out how to test monitoring here

//func
