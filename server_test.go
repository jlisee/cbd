package cbd

import (
	//	"bytes"
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

//func
