//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	fcvsock "github.com/firecracker-microvm/firecracker-go-sdk/vsock"
)

type VsockMessage struct {
	Type      string                 `json:"type"`
	VMID      string                 `json:"vm_id,omitempty"`
	Timestamp float64                `json:"timestamp,omitempty"`
	Status    string                 `json:"status,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	TaskID    string                 `json:"task_id,omitempty"`
	Result    string                 `json:"result,omitempty"`
}

func (s *scheduler) assignVsockPort(vm *VM) error {
	vm.Logger.Info("Attempting to assign vsock port")

	s.Lock()
	defer s.Unlock()

	for port := 1000; port < 1255; port++ {
		if !slices.Contains(s.assignedVsockPorts, port) {
			vm.Logger.Infof("Granting vsock port %d", port)
			vm.VsockPort = port
			s.assignedVsockPorts = append(s.assignedVsockPorts, port)
			return nil
		}
	}

	return fmt.Errorf("Could not find any available vsock ports")
}

func (vm *VM) connectToVMVsock(ctx context.Context) error {
	// Use the vsock dial function for Unix socket connection
	vsockSocketPath := filepath.Join("/srv/jailer/firecracker", vm.ID, "root/run/vsock.socket")

	vm.Logger.Infof("Connecting to VM vsock Unix socket at %s", vsockSocketPath)

	// Use the firecracker vsock dial function
	conn, err := fcvsock.DialContext(ctx, vsockSocketPath, uint32(vm.VsockPort),
		fcvsock.WithLogger(vm.Logger),
		fcvsock.WithRetryTimeout(30*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to vsock socket: %w", err)
	}

	vm.Logger.Info("Connected to VM vsock Unix socket")
	vm.VsockConn = conn
	return nil
}

func (vm *VM) vsockListener() {
	vm.Logger.Info("Listening for vsock messages")
	defer vm.VsockConn.Close()

	decoder := json.NewDecoder(vm.VsockConn)

	for {
		select {
		case <-vm.ShutdownCh:
			vm.Logger.Infof("Shutdown signal received, stopping vsockListener")
			return
		default:
			var msg VsockMessage
			if err := decoder.Decode(&msg); err != nil {
				vm.Logger.Infof("Vsock disconnected or decode error: %s", err)
				return
			}
			vm.VsockCh <- msg
		}
	}
}
