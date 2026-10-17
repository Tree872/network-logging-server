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
)

func main() {
	// Define a command-line flag for the configuration file path
	configPath := flag.String(
		"config",
		"config/server.conf",
		"Path to the server configuration file",
	)
	flag.Parse()
	// Load the server configuration from the specified file
	serverConfig, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Create the address string for the TCP listener
	address := net.JoinHostPort(
		serverConfig.Host,
		fmt.Sprintf("%d", serverConfig.Port),
	)

	// Start listening for incoming TCP connections
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start TCP listener: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Network logging server started.\n")
	fmt.Printf("Listening on TCP %s\n", listener.Addr().String())
	fmt.Printf("Configuration: %s\n", *configPath)
	fmt.Printf("Log file: %s\n", serverConfig.LogFile)
	fmt.Printf("Output format: %s\n", serverConfig.LogFormat)
	fmt.Printf("Press Ctrl+C to stop.\n")

	shutdownContext, stopSignal := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignal()

	<-shutdownContext.Done()

	fmt.Println("\nShutdown signal received.")

	err = listener.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close TCP listener: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Network logging server stopped.")
}
