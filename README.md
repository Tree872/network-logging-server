RUNNING THE SERVER

Prerequisites:
- Go must be installed.

Configure the server:
- Edit config/server.conf to configure the server address, port, log file, output format, and rate limits.

Start the server:
- Open a terminal in the project directory containing go.mod and run:
"go run ./cmd/server"

Remote connections:
- For local testing, use 127.0.0.1. For remote connections, configure the server to listen on a reachable network interface and allow inbound TCP traffic on the configured port through the firewall.
- For ZeroTier connections, both computers must join the same ZeroTier network. Connect using the server computer's ZeroTier IP address and configured port.

Log output:
- Successfully logged messages are appended to the configured log file. Supported formats are text and jsonl.
Troubleshooting:
- If the server fails to start, check the configuration and ensure the port is available. If remote connections fail, check the server address, firewall, and network connection. If messages are not saved, check the log file path, file permissions, and rate limit.