// Tests for the monitor related code.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"testing"
)

type TestListener struct {
	res  []CompletedJob    // All jobs we got
	jobs chan CompletedJob // Incoming jobs
	done chan bool         // Signal when we got a job
	run  bool
}

func newListener() *TestListener {
	l := new(TestListener)
	l.jobs = make(chan CompletedJob)
	l.done = make(chan bool)
	l.run = true

	l.runListener()

	return l
}

func (l *TestListener) runListener() {
	go func() {
		for l.run {
			completed := <-l.jobs
			l.res = append(l.res, completed)
			l.done <- true
		}
	}()
}

func TestCompletedJobPublisher(t *testing.T) {
	p := newCompletedJobPublisher()

	// Add a listener
	l1 := newListener()

	p.addObs("1", l1.jobs)

	// Publish something
	j := CompletedJob{Client: "A", Worker: "B"}
	p.publish(j)

	// Make sure we got it
	_ = <-l1.done

	if len(l1.res) != 1 {
		t.Errorf("Error with publish")
	} else {
		if l1.res[0].Client != "A" {
			t.Errorf("Error with publish")
		}
	}

	// Add another
	l2 := newListener()

	p.addObs("2", l2.jobs)

	// Publish another job
	j = CompletedJob{Client: "A", Worker: "C"}
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

	j = CompletedJob{Client: "A", Worker: "C"}
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
