// Functions and structures relating to the monitoring the state of the cluster
// This is a basic observer pattern implementation.
//
// Author: Joseph Lisee <jlisee@gmail.com>

package cbd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// ----------------------------------------------------------------------------
// Common
// ----------------------------------------------------------------------------

const (
	// Current version of the discovery packet
	discoveryVersion = uint8(1)
	discoveryMagic   = "CBD"
)

// Settings
var (
	discoveryWait = time.Duration(1) * time.Second
)

// Packet types
const (
	DISC_SERVER = uint8(iota)
	DISC_CLIENT = uint8(iota)
)

// Header of our auto-discovery packet
type discPacket struct {
	Magic   [3]uint8 // "CBD"
	Version uint8    // version
	Type    uint8    // Whether we are client or server
	Port    int32    // Desired response port, or server port
}

// Set the header to out value
func createDiscPacket(stype uint8, port int) discPacket {
	h := discPacket{
		Magic:   [3]uint8{discoveryMagic[0], discoveryMagic[1], discoveryMagic[2]},
		Version: discoveryVersion,
		Type:    stype,
		Port:    int32(port),
	}

	return h
}

// Make sure the header is valid
func (h discPacket) validate() bool {
	return discoveryMagic == string(h.Magic[:]) &&
		discoveryVersion == h.Version
}

// Write the packet to the writer
func (h discPacket) write(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, h)
}

// Read the packet from the reader
func (h *discPacket) read(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, h); err != nil {
		return err
	}

	if !h.validate() {
		return fmt.Errorf("Invalid packet, magic: %s version: %d", h.Magic,
			h.Version)
	}

	return nil
}

func isTimeout(err error) bool {
	e, ok := err.(net.Error)
	return ok && e.Timeout()
}

// ----------------------------------------------------------------------------
// Server
// ----------------------------------------------------------------------------

// Server for the auto-discovery
type discoveryServer struct {
	conn        *net.UDPConn // For listening and writing
	serverStop  chan bool    // For signaling the stopping of the server
	serverDone  chan bool    // For signaling the end of the stop
	servicePort int          // Port we sent out in the discovery packet
}

// Broadcasts on DiscoveryPort telling clients to contact on the sericePort
func newDiscoveryServer(servicePort int) (*discoveryServer, error) {
	// Allocate object
	s := new(discoveryServer)
	s.serverStop = make(chan bool)
	s.serverDone = make(chan bool)
	s.servicePort = servicePort

	// Open our connection
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),
		Port: DiscoveryPort,
	})

	if err != nil {
		return nil, err
	}

	s.conn = conn

	// Start go routine
	go s.server()

	return s, nil
}

// Launch a go routine for responding to auto discovery packets
func (s *discoveryServer) server() {
Loop:
	for {
		// See if we should be running
		select {
		case _ = <-s.serverStop:
			break Loop
		default:
			// Do nothing
		}

		// Listen at 1 Hz
		s.conn.SetReadDeadline(time.Now().Add(discoveryWait))

		// Wait for packet
		data := make([]byte, 4096)
		read, remoteAddr, err := s.conn.ReadFromUDP(data)

		if err == nil {
			// Wrap our buffer in an IO object
			rd := bytes.NewReader(data[:read])

			// Parse header
			var h discPacket

			if err := h.read(rd); err != nil {
				fmt.Println("Header read error: ", err)
				continue
			}

			// Respond to broadcaster
			remoteAddr.Port = int(h.Port)
			err = sendDiscoveryPacket(remoteAddr, s.servicePort)

			if err != nil {
				fmt.Println("Error sending response: ", err)
			}
		} else if !isTimeout(err) {
			fmt.Println("Server Error: %s\n", err)
		}
		// TODO: handle non temporary errors better
	}

	// Tell them we are done
	s.serverDone <- true
}

func (s *discoveryServer) stop() {
	s.serverStop <- true

	s.conn.Close()

	<-s.serverDone
}

// Send a discovery packet to the desired location
func sendDiscoveryPacket(addr *net.UDPAddr, myport int) error {
	// Make our UDP connection
	conn, err := net.DialUDP("udp", nil, addr)

	if err != nil {
		return err
	}

	// Create and send our header
	h := createDiscPacket(DISC_SERVER, myport)

	return h.write(conn)
}

// ----------------------------------------------------------------------------
// Client
// ----------------------------------------------------------------------------

type discResult struct {
	err  error       // nil if successful
	addr net.UDPAddr // address of remote server
}

type discClient struct {
	bconn      *net.UDPConn    // Send broadcast queries
	lconn      *net.UDPConn    // Get responses
	port       int             // Port we are listening on
	stopPing   chan bool       // Used to stop broadcasting
	stopListen chan bool       // Used to stop listening
	pingDone   chan bool       // Signals ping routine done
	listenDone chan bool       // Signals listen routine done
	result     chan discResult // Responses from the server sent on this
}

func newDiscoveryClient() (*discClient, error) {
	// Allocate object
	c := new(discClient)

	// Create channels
	c.stopPing = make(chan bool)
	c.stopListen = make(chan bool)
	c.pingDone = make(chan bool)
	c.listenDone = make(chan bool)

	// Buffered, so we can have one result ready
	c.result = make(chan discResult, 1)

	// Start the background threads
	err := c.start()

	return c, err
}

func (c *discClient) start() error {
	// Open broadcast connection
	var err error

	c.bconn, err = net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),
		Port: DiscoveryPort,
	})

	if err != nil {
		return err
	}

	// TODO: limit the listening ports to a specific range
	c.lconn, err = net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 0, // Use zero so the OS will automatically assign us a port
	})

	if err != nil {
		return err
	}

	// Figure out what port we are using for receiving UDP packets on
	addr := c.lconn.LocalAddr()
	if uaddr, ok := addr.(*net.UDPAddr); ok {
		c.port = uaddr.Port
	} else {
		return fmt.Errorf("Could not get UDPAddr from: ", addr)
	}

	// Start listening for the packets, and broadcasting responses
	go c.listen()
	go c.broadcast()

	return nil
}

// (async) Stops the broadcast and listen go-routine
func (c *discClient) stop() {
	// Signal the broadcast and listen to stop
	c.stopPing <- true
	c.stopListen <- true

	// Shutdown the port
	c.bconn.Close()
	c.lconn.Close()

	// Wait for both to stop
	<-c.pingDone
	<-c.listenDone
}

// Sends out request pings at 1 Hz
func (c *discClient) broadcast() {

	// The packet we are sending
	h := createDiscPacket(DISC_CLIENT, c.port)

Loop:
	for {
		// Send ping
		err := h.write(c.bconn)

		if err != nil {
			fmt.Println("Couldn't send packet", err)
			continue
		}

		// Launch timeout routine
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(1 * time.Second)
			timeout <- true
		}()

		// Wait for timeout or stop signal
		select {
		case <-c.stopPing:
			// Time to stop pinging
			break Loop
		case <-timeout:
			// the read from stopPing has timed out
		}
	}

	c.pingDone <- true
}

// Listens for responses from the server
func (c *discClient) listen() {
Loop:
	for {
		// Check to see if we should still run
		select {
		case _ = <-c.stopListen:
			// We got the stop message so end our loop
			break Loop
		default:
			// Do nothing
		}

		// Listen at 1 Hz
		addr, err := listenForDiscPacket(c.lconn, discoveryWait)

		if err == nil {
			// Return the result to our client or stop
			r := discResult{nil, addr}

			select {
			case c.result <- r:
				// keep going
			case _ = <-c.stopListen:
				break Loop
			}

			// Print errors
			if err != nil {
				fmt.Println("Error sending response: ", err)
			}

		} else if !isTimeout(err) {
			fmt.Println("Client Error: %s\n", err)
		}
	}

	c.listenDone <- true
}

func listenForDiscPacket(c *net.UDPConn, timeout time.Duration) (addr net.UDPAddr, err error) {
	for {
		// Listen until our deadline
		c.SetReadDeadline(time.Now().Add(timeout))

		// Wait for packet
		data := make([]byte, 4096)
		read, remoteAddr, err := c.ReadFromUDP(data)

		if err == nil {
			// Wrap our buffer in an IO object
			rd := bytes.NewReader(data[:read])

			// Parse header
			var h discPacket

			if err := h.read(rd); err != nil {
				fmt.Println("Header read error: %v", err)
				continue
			}

			// We only want server packets
			if h.Type == DISC_CLIENT {
				continue
			}

			// Return the result to our client and stop
			addr = net.UDPAddr{
				IP:   remoteAddr.IP,
				Port: int(h.Port),
				Zone: remoteAddr.Zone,
			}

			return addr, nil
		} else {
			return addr, err
		}
	}

	return addr, nil
}

// Returns the first auto-discovery server
func audoDiscoverySearch(timeout time.Duration) (string, error) {
	// Create the client, it starts pinging, and listening immediately
	c, err := newDiscoveryClient()
	defer c.stop()

	if err != nil {
		return "", err
	}

	timeoutCh := make(chan bool, 1)
	go func() {
		time.Sleep(timeout)
		timeoutCh <- true
	}()

	// Wait for timeout or stop signal
	select {
	case r := <-c.result:
		if r.err != nil {
			return "", r.err
		}

		return r.addr.String(), nil
	case <-timeoutCh:
		// the read from stopPing has timed out
		return "", fmt.Errorf("Timed out trying to read packet")
	}

	return "", fmt.Errorf("Did not get packet")
}
