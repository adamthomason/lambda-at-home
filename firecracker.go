//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"
	models "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

func newFirecrackerConfig(vm *VM) firecracker.Config {
	rootPath := filepath.Join("/srv/jailer/firecracker", vm.ID, "root")
	os.MkdirAll(rootPath, 0755)
	os.MkdirAll(filepath.Join(rootPath, "run"), 0755)

	jailerCfg := firecracker.JailerConfig{
		ID:             vm.ID,
		GID:            firecracker.Int(GID),
		UID:            firecracker.Int(UID),
		CgroupVersion:  "2",
		ExecFile:       firecrackerBinary,
		JailerBinary:   jailerBinary,
		ChrootBaseDir:  "/srv/jailer",
		NumaNode:       firecracker.Int(0),
		ChrootStrategy: firecracker.NewNaiveChrootStrategy("./" + kernelFile),
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		Stdin:          os.Stdin,
		Daemonize:      true,
	}

	fcCfg := firecracker.Config{
		VMID:            vm.ID,
		KernelImagePath: kernelPath,
		KernelArgs:      fmt.Sprintf("console=ttyS0 reboot=halt panic=-1 pci=off init=/sbin/init vm_id=%s vm_ip=%s vm_gateway=%s vsock_port=%d", vm.ID, vm.IP, hostGatewayIP, vm.VsockPort),
		SocketPath:      "./run/firecracker.socket",
		MmdsVersion:     firecracker.MMDSv2,
		Drives: []models.Drive{
			{
				DriveID:      firecracker.String("rootfs"),
				PathOnHost:   firecracker.String(rootfsPath),
				IsRootDevice: firecracker.Bool(true),
				IsReadOnly:   firecracker.Bool(false),
			},
			{
				DriveID:      firecracker.String("userCode"),
				PathOnHost:   firecracker.String(demoCodePath),
				IsRootDevice: firecracker.Bool(false),
				IsReadOnly:   firecracker.Bool(true),
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(1),
			MemSizeMib: firecracker.Int64(512),
		},
		NetworkInterfaces: firecracker.NetworkInterfaces{
			{
				StaticConfiguration: &firecracker.StaticNetworkConfiguration{
					MacAddress:  "AA:FC:00:00:00:01",
					HostDevName: vm.ID,
				},
				AllowMMDS: true,
			},
		},
		VsockDevices: []firecracker.VsockDevice{
			{
				ID:   vm.ID,
				Path: "./run/vsock.socket",
			},
		},
		LogFifo:     fmt.Sprintf("%s-logrus.fifo", vm.ID),
		MetricsFifo: fmt.Sprintf("%s-metrics.fifo", vm.ID),
		JailerCfg:   &jailerCfg,
	}

	return fcCfg
}
