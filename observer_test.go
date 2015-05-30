// Tests for the monitor related code.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"reflect"
	"testing"
)

type TestListener struct {
	res     []CompletedJob   // All jobs we got
	updates chan interface{} // Incoming Updates
	done    chan bool        // Signal when we got a job
	run     bool
}

func newListener(t *testing.T) *TestListener {
	l := new(TestListener)
	l.updates = make(chan interface{})
	l.done = make(chan bool)
	l.run = true

	l.runListener(t)

	return l
}

func (l *TestListener) runListener(t *testing.T) {
	go func() {
		for l.run {
			update := <-l.updates

			switch u := update.(type) {
			case CompletedJob:
				l.res = append(l.res, u)
			default:
				t.Errorf("Did not understand update type: " + reflect.TypeOf(update).Name())
			}

			l.done <- true
		}
	}()
}

func TestCompletedJobPublisher(t *testing.T) {
	p := newUpdatePublisher()

	// Add a listener
	l1 := newListener(t)

	p.addObs("1", l1.updates)

	// Define names
	an := MachineName{
		ID:   MachineID(""),
		Host: "A",
	}

	bn := MachineName{
		ID:   MachineID(""),
		Host: "B",
	}

	cn := MachineName{
		ID:   MachineID(""),
		Host: "C",
	}

	// Publish something
	j := CompletedJob{Client: an, Worker: bn}
	p.publish(j)

	// Make sure we got it
	_ = <-l1.done

	if len(l1.res) != 1 {
		t.Errorf("Error with publish")
	} else {
		if l1.res[0].Client != an {
			t.Errorf("Error with publish")
		}
	}

	// Add another
	l2 := newListener(t)

	p.addObs("2", l2.updates)

	// Publish another job
	j = CompletedJob{Client: an, Worker: cn}
	p.publish(j)

	_ = <-l1.done
	_ = <-l2.done

	if len(l1.res) != 2 {
		t.Errorf("Error with publish")
	}

	if len(l2.res) != 1 {
		t.Errorf("Error with publish")
	}

	// Now remove the main one
	p.removeObs("1")

	j = CompletedJob{Client: an, Worker: cn}
	p.publish(j)

	_ = <-l2.done

	if len(l1.res) != 2 {
		t.Errorf("Error with publish")
	}

	if len(l2.res) != 2 {
		t.Errorf("Error with publish")
	}

	// Halt all the go routines
	l1.run = false
	l2.run = false
}
