// Here we define the the light weight message system we use to communicate
// between machines.  This implements a very standard length + type prefixed
// message system.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type MessageID int

// Our list of message types, use to figure out what exactly each type of
// message is
const (
	CompileJobID MessageID = iota
	CompileResultID
)

var messageIDNames = [...]string{
	"CompileJobID",
	"CompileResultID",
}

func (mID MessageID) String() string {
	if int(mID) >= len(messageIDNames) {
		return "ERROR ID out of range"
	}

	return messageIDNames[mID]
}

// DeadlineReadWriter is an interface that lets you read and write
// with a possible timeut based on deadlines.
type DeadlineReadWriter interface {
	io.Reader
	io.Writer

	// SetReadDeadline sets the deadline for future Read calls.
	// A zero value for t means Read will not time out.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for future Write calls.
	// Even if write times out, it may return n > 0, indicating that
	// some of the data was successfully written.
	// A zero value for t means Write will not time out.
	SetWriteDeadline(t time.Time) error
}

// The MessageHeader procedes each message in our data stream, it lets us
// determine what exact type is in the nessage
type MessageHeader struct {
	ID MessageID
}

// A connection which you can send and receive messages over
type MessageConn struct {
	conn    DeadlineReadWriter // Sink and source for all data
	dec     *gob.Decoder       // Encodes data into our buffer
	enc     *gob.Encoder       // Decodes data into our buffer
	timeout time.Duration      // nanosecond timeout
}

// Create a TCP based message conn
func NewTCPMessageConn(address string, d time.Duration) (*MessageConn, error) {
	// Add default port if needed
	if strings.Index(address, ":") < 0 {
		address = address + ":" + strconv.Itoa(Port)
	}

	// Make our connection
	conn, err := net.Dial("tcp", address)

	if err != nil {
		return nil, err
	}

	return NewMessageConn(conn, time.Duration(10)*time.Second), nil
}

// Create a message connection with the given buffer
func NewMessageConn(c DeadlineReadWriter, d time.Duration) *MessageConn {
	m := new(MessageConn)

	m.conn = c
	m.dec = gob.NewDecoder(m.conn)
	m.enc = gob.NewEncoder(m.conn)
	m.timeout = d

	return m
}

// Generic send function, makes it simpler to send messages
func (mc MessageConn) Send(i interface{}) (err error) {
	mc.conn.SetWriteDeadline(time.Now().Add(mc.timeout))

	switch m := i.(type) {
	case CompileJob:
		err = mc.sendHeader(CompileJobID)
		if err == nil {
			return mc.enc.Encode(m)
		}
	case CompileResult:
		err = mc.sendHeader(CompileResultID)
		if err == nil {
			return mc.enc.Encode(m)
		}
	default:
		return errors.New("Could not decode type: " + reflect.TypeOf(i).Name())
	}

	return err
}

func (mc MessageConn) setReadDeadline() error {
	return mc.conn.SetReadDeadline(time.Now().Add(mc.timeout))
}

// Read the header and check the message ID
func (mc MessageConn) readHeader(eID MessageID) (h MessageHeader, err error) {
	err = mc.dec.Decode(&h)

	if err != nil {
		return h, err
	}

	// Check ID
	if eID != h.ID {
		errors.New(fmt.Sprintf("Expected type: '%s'(%d) got '%s'(%d)",
			eID, eID.String(),
			h.ID, h.ID.String()))
	}

	return h, err
}

func (mc MessageConn) sendHeader(mID MessageID) (err error) {
	// Send the header
	h := MessageHeader{
		ID: mID,
	}

	return mc.enc.Encode(h)
}

// Here we have a bunch of message functions (boo on lack of Go generics)

func (mc MessageConn) ReadCompileJob() (j CompileJob, err error) {
	err = mc.setReadDeadline()

	if err != nil {
		return j, err
	}

	_, err = mc.readHeader(CompileJobID)

	if err != nil {
		return j, err
	}

	err = mc.dec.Decode(&j)
	return j, err
}

func (mc MessageConn) ReadCompileResult() (r CompileResult, err error) {
	err = mc.setReadDeadline()

	if err != nil {
		return r, err
	}

	_, err = mc.readHeader(CompileJobID)

	if err != nil {
		return r, err
	}

	err = mc.dec.Decode(&r)
	return r, err
}
