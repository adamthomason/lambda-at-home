#!/bin/sh
echo "Init script starting"

# Mount /proc if it's not already mounted
if [ ! -f /proc/cmdline ]; then
    echo "Mounting /proc..."
    mount -t proc proc /proc
fi

CMDLINE=$(cat /proc/cmdline)

# Extract values
VM_ID=$(echo "$CMDLINE" | sed -n 's/.*vm_id=\([^ ]*\).*/\1/p')
VM_IP=$(echo "$CMDLINE" | sed -n 's/.*vm_ip=\([^ ]*\).*/\1/p')
VM_GATEWAY=$(echo "$CMDLINE" | sed -n 's/.*vm_gateway=\([^ ]*\).*/\1/p')
VSOCK_PORT=$(echo "$CMDLINE" | sed -n 's/.*vsock_port=\([^ ]*\).*/\1/p')

echo "VM ID: $VM_ID"
echo "VM IP: $VM_IP"
echo "VM Gateway: $VM_GATEWAY"
echo "VSOCK Port: $VSOCK_PORT"

# Mount the userCode drive
echo "Mounting code drive"
mkdir -p /mnt/code
mount /dev/vdb /mnt/code

if [ $? -eq 0 ]; then
    echo "Successfully mounted userCode drive" > /dev/ttyS0
else
    echo "Failed to mount userCode drive" > /dev/ttyS0
    exit 1
fi

echo "Configuring host connectivity"
busybox ip addr add "$VM_IP/24" dev eth0
busybox ip link set eth0 up
busybox ip route add default via "$VM_GATEWAY"

echo "Executing runtime"
export PYTHONUNBUFFERED=1
exec runtime $VM_ID $VM_IP $VM_GATEWAY $VSOCK_PORT > /dev/ttyS0 2>&1
