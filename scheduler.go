//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
)

const (
	apiPort    = 5000
	eventsPort = 8090
	logsPort   = 3000

	firecrackerBinary = "/usr/local/bin/firecracker"
	jailerBinary      = "/usr/local/bin/jailer"

	kernelFile = "vmlinux-6.1.128"
	kernelPath = "/opt/lambda-at-home/" + kernelFile

	rootfsFile = "lambda-at-home-python-3.13.ext4"
	rootfsPath = "/opt/lambda-at-home/" + rootfsFile

	demoCodeFile = "python-3.13-example.ext4"
	demoCodePath = "/opt/lambda-at-home/" + demoCodeFile

	UID = 1000
	GID = 1000
)

type Log struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type LogsResponse struct {
	Logs        []Log  `json:"logs"`
	Count       int    `json:"count"`
	RetrievedAt string `json:"retrieved_at"`
}

type scheduler struct {
	vms                map[string]*VM
	assignedVsockPorts []int
	creationAllowed    bool
	sync.Mutex
	shutdownCh chan *VM
}

func main() {
	s := scheduler{
		creationAllowed: true,
		vms:             make(map[string]*VM),
		shutdownCh:      make(chan *VM),
	}

	go s.eventListener()
	go s.shutdownListener()

	go func() {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// Wait for interrupt
		<-ctx.Done()

		logrus.Info("Shutdown signal received. Checking for running VMs.")
		s.creationAllowed = false
		ticker := time.NewTicker(1 * time.Second)

		for range ticker.C {
			remainingVms := len(s.vms)

			if remainingVms == 0 {
				logrus.Info("All VMs exited. Exiting program.")
				os.Exit(0)
			}

			logrus.Infof("Waiting for %d VMs to exit", remainingVms)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("POST /vm", func(w http.ResponseWriter, r *http.Request) {
		vm, err := s.createVm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(vm)
	})

	logrus.Infof("Scheduler API listening on port %d", apiPort)
	logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", apiPort), mux))
}

func (s *scheduler) createVm() (VM, error) {
	if !s.creationAllowed {
		return VM{}, fmt.Errorf("VM creation is not allowed currently.")
	}

	ctx := context.Background()

	baseLogger := logrus.New()
	baseLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	baseLogger.SetLevel(logrus.WarnLevel)

	vm, err := newVM(baseLogger)

	err = s.createTapInterface(&vm)
	if err != nil {
		return VM{}, err
	}

	err = s.assignVsockPort(&vm)
	if err != nil {
		return VM{}, err
	}

	fcCfg := newFirecrackerConfig(&vm)

	vm.Logger.Debugf("Creating Firecracker machine")
	machine, err := firecracker.NewMachine(
		ctx,
		fcCfg,
		firecracker.WithLogger(vm.Logger),
	)
	if err != nil {
		vm.Logger.Errorf("Failed to configure Firecracker machine: %v", err)
		s.deleteVm(ctx, &vm)
		return VM{}, err
	}
	vm.Machine = machine

	err = vm.Machine.Start(ctx)
	if err != nil {
		vm.Logger.Errorf("Failed to start machine: %v", err)
		s.deleteVm(ctx, &vm)
		return VM{}, err
	}

	vm.StartTime = time.Now()

	go vm.lifecycleManager(ctx, &s.shutdownCh)

	return vm, nil
}

func (s *scheduler) shutdownListener() {
	ctx := context.Background()

	for vm := range s.shutdownCh {
		vm.Logger.Info("Received shutdown request")
		go s.deleteVm(ctx, vm)
	}
}

func (s *scheduler) deleteVm(ctx context.Context, vm *VM) error {
	vm.Logger.Warn("Attempting to shut down VM")
	err := vm.Machine.Shutdown(ctx)
	if err != nil {
		vm.Logger.Errorf("Could not shut down machine: %v", err)
	}

	vm.EndTime = time.Now()

	err = vm.deleteTapInterface()
	if err != nil {
		return err
	}

	s.Lock()
	delete(s.vms, vm.IP)

	for i, v := range s.assignedVsockPorts {
		if v == vm.VsockPort {
			s.assignedVsockPorts = append(s.assignedVsockPorts[:i], s.assignedVsockPorts[i+1:]...)
			break
		}
	}

	s.Unlock()

	return nil
}
