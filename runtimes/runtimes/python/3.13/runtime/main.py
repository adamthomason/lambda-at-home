import logging
import signal
import sys
import importlib.util
import time
import socket
import json
from typing import Optional
import threading
import datetime
from collections import deque


class LogBufferHandler(logging.Handler):
    """
    Custom logging handler which buffers logs awaiting to be emitted
    """

    def __init__(self, max_size: int = 1000):
        super().__init__()
        self.buffer = deque(maxlen=max_size)
        self.lock = threading.RLock()

    def emit(self, record: logging.LogRecord):
        """Store log record in buffer."""

        with self.lock:
            log_entry = {
                "timestamp": datetime.datetime.fromtimestamp(
                    record.created
                ).isoformat(),
                "level": record.levelname,
                "logger": record.name,
                "message": self.format(record),
                "module": record.module,
                "function": record.funcName,
                "line": record.lineno,
            }

            self.buffer.append(log_entry)

    def get_logs(self, count: int = 100) -> list[dict[str, any]]:
        """Retrieve logs from buffer and remove them."""
        with self.lock:
            # If there are no logs, just return empty list
            if not self.buffer:
                return []

            buffer_size = len(self.buffer)

            # If there are fewer logs than count, return the whole buffer and clear
            if buffer_size <= count:
                logs = list(self.buffer)
                self.buffer.clear()
                return logs

            # If there are more logs than count, return count of logs, and pop them
            logs = []
            for _ in range(count):
                logs.append(self.buffer.popleft())

            return logs


class App:
    def __init__(self, id: str, gateway: str):
        self.id = id
        self.gateway = gateway

        # Create logger with buffer handler
        self.logger = logging.getLogger(f"app.{id}")
        self.logger.setLevel(logging.DEBUG)

        # Create and configure buffer handler
        # self.buffer_handler = LogBufferHandler(max_size=1000)
        # formatter = logging.Formatter(
        #     "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
        # )
        # self.buffer_handler.setFormatter(formatter)
        # self.logger.addHandler(self.buffer_handler)

        self.logger.info(f"App initialized with ID: {id}")


class Runtime:
    def __init__(
        self,
        id: str,
        ip: str,
        gateway: str,
        vsock_port: str,
    ):
        self.logger = logging.getLogger("runtime")
        self.id = id
        self.ip = ip
        self.gateway = gateway
        self.vsock_port = vsock_port

        self.vhost_socket: Optional[socket.socket] = None
        self.host_cid = 2
        self.af_vsock = 40
        self.vsock_port = vsock_port

        self.app = App(self.id, self.gateway)
        self.setup_signal_handlers()
        self.handler_module = None
        self.load_handler()

        self.running = True

    def setup_vsock_server(self) -> bool:
        """Set up vsock server to handle host connections."""
        try:
            # Create a vsock server socket that listens on the assigned port
            self.vhost_socket = socket.socket(socket.AF_VSOCK, socket.SOCK_STREAM)

            # Bind to any CID and the assigned port
            self.vhost_socket.bind((socket.VMADDR_CID_ANY, int(self.vsock_port)))
            self.vhost_socket.listen(1)

            self.logger.info(
                f"Listening for host connection on vsock port {self.vsock_port}"
            )

            # Accept the connection from host
            conn, addr = self.vhost_socket.accept()
            self.vhost_socket = conn  # Replace listener socket with connection socket

            self.logger.info(f"Host connected from CID {addr[0]}")
            return True

        except Exception as e:
            self.logger.error(
                f"Failed to set up vsock server on port {self.vsock_port}: {e}"
            )
            if self.vhost_socket:
                self.vhost_socket.close()
                self.vhost_socket = None
            return False

    def send_vsock_message(self, message: str) -> Optional[str]:
        """Send a message to the host and return the response."""
        if not self.vhost_socket:
            self.logger.error("Not connected to host")
            return None

        self.vhost_socket.sendall((message).encode())

        # Receive a response
        buffer = b""
        while True:
            data = self.vhost_socket.recv(1024)
            if not data:
                break
            buffer += data
            if b"\n" in buffer:
                break

        response = buffer.decode().strip()
        return response

    def send_vsock_json(self, data: dict) -> Optional[dict]:
        """Send JSON data and return JSON response."""
        try:
            message = json.dumps(data) + "\n"
            response = self.send_vsock_message(message)
            if response:
                return json.loads(response)
        except json.JSONDecodeError as e:
            self.logger.error(f"JSON decode error: {e}")
        return None

    def disconnect_vsock(self):
        """Disconnect from the host."""
        if self.vhost_socket:
            self.vhost_socket.close()
            self.vhost_socket = None
            self.logger.info("Disconnected from host")

    def load_handler(self):
        """Load the handler module dynamically"""
        try:
            spec = importlib.util.spec_from_file_location(
                "handler", "/mnt/code/handler.py"
            )
            self.handler_module = importlib.util.module_from_spec(spec)
            sys.modules["handler"] = self.handler_module
            spec.loader.exec_module(self.handler_module)
            self.logger.debug("Handler module loaded successfully")
        except Exception as e:
            self.logger.warning(f"Warning: Could not load handler module: {e}")
            sys.exit(1)

    def setup_signal_handlers(self):
        """Set up signal handlers for graceful shutdown"""
        signal.signal(signal.SIGTERM, self.signal_handler)
        signal.signal(signal.SIGINT, self.signal_handler)
        signal.signal(signal.SIGHUP, self.signal_handler)

    def signal_handler(self, signum, frame):
        """Handle shutdown signals gracefully"""
        raise InterruptedError(f"Received signal {signum}")

    def signal_completion(self):
        res = self.send_vsock_json({"type": "done", "timestamp": time.time()})

        if res and res.get("type") == "done_ack":
            return True

        return False

    def call_handler(self, app, *args, **kwargs):
        """Call the handler function with arguments"""
        if self.handler_module and hasattr(self.handler_module, "handler"):
            try:
                result = self.handler_module.handler(app, *args, **kwargs)

                return result
            except SystemExit as e:
                self.logger.error(f"Handler called sys.exit({e.code})")
                return None
            except Exception as e:
                self.logger.error(f"Error calling handler: {e}")
                return None
        else:
            raise FileNotFoundError("Could not find handler")

    def run(self):
        """Main application logic"""
        self.running = True

        # Set up vsock server instead of connecting
        vsock_connected = False
        while not vsock_connected:
            if self.setup_vsock_server():
                vsock_connected = True
                break

            self.logger.error("Failed to set up vsock server, retrying in 5 seconds...")
            time.sleep(5)
            continue

        try:
            res = self.send_vsock_json({"type": "ready", "timestamp": time.time()})

            if res and res.get("type") == "ready_ack":
                self.call_handler(self.app)

                if not self.signal_completion():
                    raise Exception("Did not receive completion acknowledgment")
            else:
                raise Exception("Did not receive ready acknowledgement from scheduler")
        except InterruptedError as e:
            self.logger.warning(e)
            self.logger.info("Shutting down gracefully")
        except Exception as e:
            self.logger.error(f"Error in main loop: {e}")
            self.running = False
        finally:
            self.disconnect_vsock()

        # Wait for shutdown
        while self.running:
            time.sleep(1)


def main():
    runtime = None

    try:
        args = sys.argv
        runtime = Runtime(
            id=args[1],
            ip=args[2],
            gateway=args[3],
            vsock_port=args[4],
        )

        runtime.run()

    except Exception as e:
        if runtime:
            runtime.logger.error(f"Error in main: {e}")

    finally:
        if runtime:
            runtime.logger.info("VM shutting down cleanly")


if __name__ == "__main__":
    main()
