//go:build linux

package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

type vmEvent struct {
	payload string
}

func (s *scheduler) eventListener() {
	addr := net.UDPAddr{
		Port: eventsPort,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		logrus.Fatalf("Failed to bind event UDP listener: %v", err)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)

	bindAddr := fmt.Sprintf("%s:%d", addr.IP.String(), eventsPort)
	logrus.Infof("Event listener listening on %s", bindAddr)
	for {
		n, senderAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			logrus.Errorf("Error reading UDP message: %v", err)
			continue
		}
		msg := strings.TrimSpace(string(buffer[:n]))

		// Message should be e.g "vm-1234:done"
		parts := strings.Split(msg, ":")
		if len(parts) != 2 {
			logrus.Warnf("Invalid message format: %s (expected format: vmid:event)", msg)
			continue
		}

		senderIP := senderAddr.IP.String()
		vmID := parts[0]
		eventPayload := parts[1]

		s.Lock()
		vm, ok := s.vms[senderIP]
		s.Unlock()

		if !ok {
			logrus.Warnf("Received event from unknown IP: %s", senderIP)
			continue
		}

		if vm.ID != vmID {
			logrus.Warnf("Received event for unknown VM ID: %s", vmID)
			continue
		}

		logrus.Debugf("Dispatching event '%s' to VM %s", eventPayload, vm.ID)
		select {
		case vm.EventCh <- vmEvent{payload: eventPayload}:
			logrus.Debugf("Successfully dispatched event to VM %s", vm.ID)
		default:
			logrus.Warnf("Failed to dispatch event to VM %s (channel full or closed)", vm.ID)
		}
	}
}
