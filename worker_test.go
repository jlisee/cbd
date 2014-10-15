package cbd

import (
	"testing"
	"time"
)

func TestSendWorkerState(t *testing.T) {
	// Create and check our worker
	w, err := NewWorker(57, "server:89")

	if err != nil {
		t.Error("Making worker:", err)
		return
	}

	if 57 != w.port {
		t.Error("Port incorrect")
	}

	// Now lets test things
	var network MockConn
	mc := NewMessageConn(&network, time.Duration(10)*time.Second)

	// We set this to false so the background thread only makes one pass
	w.run = false
	w.sendWorkerState(mc, "bob")

	s, err := mc.ReadWorkerState()

	if err != nil {
		t.Error("Reading worker state:", err)
		return
	}

	if s.Port != 57 {
		t.Error("Port incorrect")
	}

	if s.Host != "bob" {
		t.Errorf("Got host \"%s\" wanted %s", s.Host, "bob")
	}

	if s.Load <= 0 {
		t.Errorf("Bad system load")
	}
}
