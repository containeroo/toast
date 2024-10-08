package checker

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const (
	envICMPReadTimeout string = "ICMP_READ_TIMEOUT"

	defaultICMPReadTimeout time.Duration = time.Second * 1
)

// ICMPChecker implements a basic ICMP ping checker.
type ICMPChecker struct {
	Name         string        // The name of the checker.
	Address      string        // The address of the target.
	Protocol     Protocol      // The protocol to use for the connection.
	ReadTimeout  time.Duration // The timeout for reading the ICMP reply.
	WriteTimeout time.Duration // The timeout for writing the ICMP request.
}

// String returns the name of the checker.
func (c *ICMPChecker) String() string {
	return c.Name
}

// NewICMPChecker initializes a new ICMPChecker with its specific configuration.
func NewICMPChecker(name, address string, dialTimeout time.Duration, getEnv func(string) string) (Checker, error) {
	// The "icmp://" prefix is used to identify the check type and is not needed for further processing,
	// so it must be removed before passing the address to other functions.
	address = strings.TrimPrefix(address, "icmp://")

	checker := ICMPChecker{
		Name:         name,
		Address:      address,
		ReadTimeout:  defaultICMPReadTimeout,
		WriteTimeout: dialTimeout,
	}

	protocol, err := newProtocol(checker.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to create ICMP protocol: %w", err)
	}
	checker.Protocol = protocol

	// Determine the read timeout
	if readTimeoutStr := getEnv(envICMPReadTimeout); readTimeoutStr != "" {
		readTimeout, err := time.ParseDuration(readTimeoutStr)
		if err != nil || readTimeout <= 0 {
			return nil, fmt.Errorf("invalid %s value: %s", envICMPReadTimeout, readTimeoutStr)
		}
		checker.ReadTimeout = readTimeout
	}

	return &checker, nil
}

// Check performs an ICMP check on the target.
func (c *ICMPChecker) Check(ctx context.Context) error {
	// Resolve the IP address
	dst, err := net.ResolveIPAddr(c.Protocol.Network(), c.Address)
	if err != nil {
		return fmt.Errorf("failed to resolve IP address: %w", err)
	}

	// Listen for ICMP packets
	conn, err := c.Protocol.ListenPacket(ctx, c.Protocol.Network(), "0.0.0.0")
	if err != nil {
		return fmt.Errorf("failed to listen for ICMP packets: %w", err)
	}
	defer conn.Close()

	identifier := uint16(os.Getpid() & 0xffff)                    // Create a unique identifier
	sequence := uint16(atomic.AddUint32(new(uint32), 1) & 0xffff) // Create a unique sequence number

	// Make the ICMP request
	msg, err := c.Protocol.MakeRequest(identifier, sequence)
	if err != nil {
		return err
	}

	// Write the ICMP request
	if err := c.writeICMPRequest(ctx, conn, msg, dst); err != nil {
		return err
	}

	// Read the ICMP reply with context
	reply, err := c.readICMPReply(ctx, conn)
	if err != nil {
		return err
	}

	// Validate the ICMP reply
	if err := c.validateICMPReply(ctx, reply, identifier, sequence); err != nil {
		return err
	}

	return nil
}

// writeICMPRequest handles writing the ICMP request.
func (c *ICMPChecker) writeICMPRequest(ctx context.Context, conn net.PacketConn, msg []byte, dst net.Addr) error {
	if err := conn.SetWriteDeadline(time.Now().Add(c.ReadTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	done := make(chan error, 1)

	go func() {
		_, err := conn.WriteTo(msg, dst)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to send ICMP request to %s: %w", c.Address, err)
		}
		return nil
	}
}

// readICMPReply handles reading the ICMP reply.
func (c *ICMPChecker) readICMPReply(ctx context.Context, conn net.PacketConn) ([]byte, error) {
	// Set the read deadline
	if err := conn.SetReadDeadline(time.Now().Add(c.ReadTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	done := make(chan error, 1)
	reply := make([]byte, 1500)
	var n int

	go func() {
		var err error
		n, _, err = conn.ReadFrom(reply)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("failed to read ICMP reply from %s: %w", c.Address, err)
		}
		return reply[:n], nil
	}
}

// validateICMPReply handles validating the ICMP reply.
func (c *ICMPChecker) validateICMPReply(ctx context.Context, reply []byte, identifier, sequence uint16) error {
	done := make(chan error, 1)

	go func() {
		err := c.Protocol.ValidateReply(reply, identifier, sequence)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return err
		}
		return nil
	}
}
