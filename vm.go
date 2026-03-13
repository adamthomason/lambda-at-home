//go:build linux

package main

import (
	"context"
	"encoding/json"
	"net"
	"time"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
)

type VM struct {
	ID         string               `json:"id"`
	StartTime  time.Time            `json:"startTime"`
	EndTime    time.Time            `json:"endTime"`
	Logs       []Log                `json:"-"`
	Logger     *logrus.Entry        `json:"-"`
	Machine    *firecracker.Machine `json:"-"`
	UID        int                  `json:"-"`
	GID        int                  `json:"-"`
	IP         string               `json:"-"`
	ShutdownCh chan struct{}        `json:"-"`
	EventCh    chan vmEvent         `json:"-"`
	VsockConn  net.Conn             `json:"-"`
	VsockCh    chan VsockMessage    `json:"-"`
	VsockPort  int                  `json:"-"`
}

func newVM(baseLogger *logrus.Logger) (VM, error) {
	id, err := generateId()
	if err != nil {
		return VM{}, err
	}

	vm := VM{
		ID:         id,
		Logger:     baseLogger.WithField("vm_id", id),
		UID:        1000,
		GID:        1000,
		EventCh:    make(chan vmEvent),
		VsockCh:    make(chan VsockMessage),
		ShutdownCh: make(chan struct{}),
	}

	return vm, nil
}

func (vm *VM) lifecycleManager(ctx context.Context, schedulerShutdownCh *chan *VM) {
	vm.Logger.Debugf("VM handed to lifecycle manager")

	future := time.Now().Add(60 * time.Second)
	timeout, cancel := context.WithDeadline(ctx, future)

	defer close(vm.ShutdownCh)
	defer cancel()
	defer close(vm.EventCh)
	defer close(vm.VsockCh)
	defer func() {
		vm.Logger.Warn("Running deferred shutdown channel send")
		*schedulerShutdownCh <- vm
	}()

	err := vm.connectToVMVsock(ctx)
	if err != nil {
		vm.Logger.Errorf("Failed to connect to Vsock: %v", err)
		return
	}
	defer vm.VsockConn.Close()

	vsockEncoder := json.NewEncoder(vm.VsockConn)

	go vm.vsockListener()

	logFetchTicker := time.NewTicker(5 * time.Second)
	defer logFetchTicker.Stop()

	// Main lifecycle loop
	for {
		select {
		case <-timeout.Done():
			vm.Logger.Info("Timeout reached before task completion. Shutting down gracefully.")
			return

		case <-vm.EventCh:
			vm.Logger.Info("Task completion event received. Shutting down gracefully after 2 seconds.")
			time.Sleep(2 * time.Second)
			return

		case msg := <-vm.VsockCh:
			vm.Logger.Warnf("VSOCK_MESSAGE: %s", msg.Type)

			switch msg.Type {

			case "ready":
				vsockEncoder.Encode(VsockMessage{Type: "ready_ack"})

			case "done":
				vsockEncoder.Encode(VsockMessage{Type: "done_ack"})
				return

			default:
				vm.Logger.Warnf("VSOCK_MESSAGE: unknown (%s)", msg.Type)
			}
		}
	}
}
