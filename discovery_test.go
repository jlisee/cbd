package cbd

import (
	"bytes"
	"fmt"
	"net"
	"testing"
	"time"
)

// This forces the background loops to run quickly, meaning the check for exit
// fast, allowing tests to finish in a timely manner.
func discTestSetup() {
	discoveryWait = time.Duration(10) * time.Millisecond
}

func TestDiscPacket(t *testing.T) {
	discTestSetup()

	// Create packet and make sure it validates
	p := createDiscPacket(DISC_CLIENT, 123)

	if !p.validate() {
		t.Errorf("Failed to validate")
	}

	// Read and write from the packet from the buffer
	var p2 discPacket

	wbuf := new(bytes.Buffer)
	err := p.write(wbuf)

	if err != nil {
		t.Errorf("Error writing data: %s", err)
	}

	rbuf := bytes.NewBuffer(wbuf.Bytes())
	err = p2.read(rbuf)

	if err != nil {
		t.Errorf("Error reading data: %s", err)
	}

	// Check everything
	if p.Version != p2.Version {
		t.Errorf("Version mismatch, got %d want %d", p2.Version, p.Version)
	}

	if 123 != p2.Port {
		t.Errorf("Port not set right, got %d want %d", p2.Port, 123)
	}

	magStr := string([]byte(p2.Magic[:]))
	if "CBD" != magStr {
		t.Errorf("Got magic: '%s' want '%s'", magStr)
	}
}

// Make sure we can stop the discovery server
func TestDiscoveryStop(t *testing.T) {
	discTestSetup()

	s, err := newDiscoveryServer(123)

	if err != nil {
		t.Errorf("Could not create server", err)
		return
	}

	s.stop()
}

// Make sure we can stop the discovery client
func TestDiscoverClientStop(t *testing.T) {
	discTestSetup()

	c, err := newDiscoveryClient()

	if err != nil {
		t.Errorf("Could not create client", err)
	}

	c.stop()
}

// Return the IP address we expect to get from the discovery server
func getExpIp(port int) net.UDPAddr {
	ips, _ := getLocalIPAddrs()

	return net.UDPAddr{
		IP:   ips[0].IP,
		Port: port,
		Zone: "",
	}
}

func TestDiscoverySearch(t *testing.T) {
	discTestSetup()

	// Make the server
	s, err := newDiscoveryServer(123)

	if err != nil {
		t.Errorf("Could not create server", err)
		return
	}

	// Make the client
	c, err := newDiscoveryClient()

	if err != nil {
		t.Error("Could not create client", err)
		s.stop()
		return
	}

	// See what happens!
	r := <-c.result

	// Form the expected result. Assume the first interface is the one we want.
	exp := getExpIp(123)

	// Compare results. We are using String here because for DeepEqual is saying
	// to IPs which look the same, are different.
	if exp.String() != r.addr.String() {
		t.Errorf("Wanted discovery: %+v, got: %+v", exp, r.addr)
	}

	// Stop everything
	c.stop()
	s.stop()
}

func TestDiscoverySearchFunc(t *testing.T) {
	discTestSetup()

	// No server search
	_, err := audoDiscoverySearch(time.Duration(10) * time.Millisecond)

	if err == nil {
		t.Errorf("Should of gotten an error")
	}

	// Make the server
	s, err := newDiscoveryServer(123)

	if err != nil {
		t.Errorf("Could not create server", err)
		return
	}

	// Search for it
	addrStr, err := audoDiscoverySearch(time.Duration(1) * time.Second)

	if err != nil {
		t.Errorf("Failed to find discovery server: %v", err)
		return
	}

	// Compare to our expected string
	expIp := getExpIp(123)

	expStr := fmt.Sprintf("%v:%d", expIp.IP, expIp.Port)

	// Compare results, make sure things match properly
	if expStr != addrStr {
		t.Errorf("Wanted discovery: %+v, got: %+v", expStr, addrStr)
	}

	// Shutdown things
	s.stop()
}
