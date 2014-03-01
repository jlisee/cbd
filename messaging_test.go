package cbd

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"
)

// MockConn is a buffer with blank deadlines
type MockConn struct {
	bytes.Buffer
}

func (m MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// Here we just make sure we can send back and forth messages
func TestMessageConn(t *testing.T) {
	var network MockConn
	mc := NewMessageConn(&network, time.Duration(10)*time.Second)

	input := CompileResult{
		ExecResult: ExecResult{
			Return: 5,
			Output: []byte("Awesome."),
		},
		ObjectCode: []byte("1 + 1 = 3"),
	}

	mc.Send(input)
	output, err := mc.ReadCompileResult()

	if err != nil {
		t.Error("read:", err)
		return
	}

	// Just check one field
	if input.Return != output.Return {
		t.Errorf("Return not serialized")
	}
}

func TestGobEncoding(t *testing.T) {
	var buf bytes.Buffer

	input := CompileResult{
		ExecResult: ExecResult{
			Return: 5,
			Output: []byte("Awesome."),
		},
		ObjectCode: []byte("1 + 1 = 3"),
	}

	// Create an encoder and send a value.
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(input)

	if err != nil {
		t.Error("encode:", err)
		return
	}

	// Create a decoder and receive a value.
	dec := gob.NewDecoder(&buf)

	var output CompileResult
	err = dec.Decode(&output)

	if err != nil {
		t.Error("decode:", err)
		return
	}

	// Now check the results
	if input.Return != output.Return {
		t.Errorf("Return not serialized")
	}

	if 0 != bytes.Compare(input.Output, output.Output) {
		t.Errorf("Output not serialized properly")
	}

	if 0 != bytes.Compare(input.ObjectCode, output.ObjectCode) {
		t.Errorf("ObjectCode not serialized properly")
	}
}
