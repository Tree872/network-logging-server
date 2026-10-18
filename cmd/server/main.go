// File: main.go
// Description: This file contains the main entry point for the network logging server application.

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"network-logging-server/internal/config"
	"network-logging-server/internal/logging"
	"network-logging-server/internal/server"
)

func main() {
	var configPath *string
	configPath = flag.String(
		"config",
		"config/server.conf",
		"Path to the server configuration file",
	)
	flag.Parse()

	var serverConfig config.Config
	var err error

	serverConfig, err = config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	var logWriter *logging.Logger
	logWriter, err = logging.New(
		serverConfig.LogFile,
		serverConfig.LogFormat,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	var address string
	address = net.JoinHostPort(
		serverConfig.Host,
		fmt.Sprintf("%d", serverConfig.Port),
	)

	var listener net.Listener
	listener, err = net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start TCP listener: %v\n", err)
		logWriter.Close()
		os.Exit(1)
	}

	fmt.Println("Network logging server started.")
	fmt.Printf("Listening on TCP %s\n", listener.Addr().String())
	fmt.Printf("Configuration: %s\n", *configPath)
	fmt.Printf("Log file: %s\n", serverConfig.LogFile)
	fmt.Printf("Output format: %s\n", serverConfig.LogFormat)
	fmt.Println("Press Ctrl+C to stop.")

	var shutdownContext context.Context
	var stopSignal context.CancelFunc

	shutdownContext, stopSignal = signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignal()

	err = server.Run(
		listener,
		shutdownContext,
		serverConfig.MaxMessageSize,
		logWriter,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		logWriter.Close()
		os.Exit(1)
	}

	err = logWriter.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close log file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Network logging server stopped.")
}
