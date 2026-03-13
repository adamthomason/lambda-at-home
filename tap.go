//go:build linux

package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

const subnet = "192.168.100."
const mask = "24"
const hostGatewayIP = subnet + "1"

func (s *scheduler) allocateAvailableIP(vm *VM) error {
	vm.Logger.Debug("Allocating IP address")

	s.Lock()
	defer s.Unlock()

	usedIPs := make([]string, len(s.vms))

	i := 0
	for k := range s.vms {
		usedIPs[i] = k
		i++
	}

	usedSet := make(map[string]bool)
	for _, ip := range usedIPs {
		usedSet[ip] = true
	}

	for i := 2; i < 255; i++ {
		ip := fmt.Sprintf("%s%d", subnet, i)
		if !slices.Contains(usedIPs, ip) && ip != hostGatewayIP {
			vm.IP = ip
			s.vms[vm.IP] = vm
			return nil
		}
	}

	return fmt.Errorf("no available IPs in subnet")
}

func (s *scheduler) createTapInterface(vm *VM) error {
	err := s.allocateAvailableIP(vm)
	if err != nil {
		return err
	}

	tap := &netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: vm.ID,
		},
		Mode:  netlink.TUNTAP_MODE_TAP,
		Flags: netlink.TUNTAP_DEFAULTS | netlink.TUNTAP_NO_PI,
		Owner: uint32(vm.UID),
		Group: uint32(vm.GID),
	}

	vm.Logger.Debug("Creating TAP interface")
	if err := netlink.LinkAdd(tap); err != nil {
		vm.Logger.Errorf("Failed to create TAP interface: %v", err)
		return err
	}

	// Only assign the HOST gateway IP to the TAP interface
	// The VM will configure its own IP internally
	hostCIDR := fmt.Sprintf("%s/%s", hostGatewayIP, mask)
	hostAddr, err := netlink.ParseAddr(hostCIDR)
	if err != nil {
		vm.Logger.Errorf("Could not parse host gateway addr %s", hostCIDR)
		vm.deleteTapInterface()
		return err
	}

	vm.Logger.Debugf("Assigning gateway IP to TAP %s: %s", tap.Name, hostCIDR)

	if err := netlink.AddrAdd(tap, hostAddr); err != nil {
		vm.Logger.Errorf("Failed to add host gateway IP: %v", err)
		vm.deleteTapInterface()
		return err
	}

	vm.Logger.Debugf("Bringing up TAP interface")
	if err := netlink.LinkSetUp(tap); err != nil {
		vm.Logger.Errorf("Could not set TAP to UP: %v", err)
		vm.deleteTapInterface()
		return err
	}

	if err := os.Chown("/sys/class/net/"+tap.Name, vm.UID, vm.GID); err != nil {
		vm.Logger.Errorf("Failed setting ownership for TAP %s: %v", tap.Name, err)
		vm.deleteTapInterface()
		return err
	}

	// Enable IP forwarding for this interface
	vm.Logger.Debugf("Enabling IP forwarding")
	if err := vm.enableIPForwarding(tap.Name); err != nil {
		vm.Logger.Errorf("Failed to enable IP forwarding: %v", err)
		// Don't fail completely, but log the error
	}

	// Set up iptables rules
	ipt, err := iptables.New()
	if err != nil {
		vm.Logger.Errorf("Failed to create iptables instance: %v", err)
		vm.deleteTapInterface()
		return err
	}

	// Allow all traffic between VM and host on this TAP interface
	if err := ipt.Append("filter", "INPUT", "-i", tap.Name, "-j", "ACCEPT"); err != nil {
		vm.Logger.Errorf("Failed to allow INPUT from TAP: %v", err)
		vm.deleteTapInterface()
		return err
	}

	if err := ipt.Append("filter", "OUTPUT", "-o", tap.Name, "-j", "ACCEPT"); err != nil {
		vm.Logger.Errorf("Failed to allow OUTPUT to TAP: %v", err)
		vm.deleteTapInterface()
		return err
	}

	return nil
}

func (vm *VM) enableIPForwarding(interfaceName string) error {
	// Enable IP forwarding for the specific interface
	procPath := fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/forwarding", interfaceName)
	return os.WriteFile(procPath, []byte("1"), 0644)
}

func (vm *VM) deleteTapInterface() error {
	vm.Logger.Debug("Deleting TAP interface")

	table := "filter"
	chains := []string{"INPUT", "FORWARD"}

	ipt, err := iptables.New()
	if err != nil {
		vm.Logger.Errorf("Failed to create iptables instance: %v", err)
		return err
	}

	for _, chain := range chains {
		rules, err := ipt.List(table, chain)
		if err != nil {
			vm.Logger.Errorf("Failed to list rules for chain %s: %v", chain, err)
			continue
		}

		for _, rule := range rules {
			if strings.Contains(rule, "-i "+vm.ID) || strings.Contains(rule, "-o "+vm.ID) {
				ipt.Delete(table, rule)
			}
		}
	}

	tap, err := netlink.LinkByName(vm.ID)
	if err != nil {
		vm.Logger.Errorf("Failed to get TAP interface: %v", err)
		return err
	}

	if err := netlink.LinkDel(tap); err != nil {
		vm.Logger.Errorf("Failed to delete TAP: %v", err)
		return err
	}

	vm.Logger.Debug("Successfully deleted TAP interface")
	return nil
}
