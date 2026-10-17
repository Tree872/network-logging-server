package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// Function: Run
// Description:
// Run starts the server to accept incoming TCP connections and handle them concurrently.
// It listens for incoming connections on the provided listener and spawns a new goroutine for each connection.
// The server continues to accept connections until a shutdown signal is received via the shutdownContext.
// Parameters:
// - listener: The net.Listener to accept incoming TCP connections.
// - shutdownContext: A context that signals when the server should shut down gracefully.
// Returns:
// - error: An error if the server encounters an issue while running.
func Run(listener net.Listener, shutdownContext context.Context) error {
	var activeConnections map[net.Conn]struct{}
	var connectionMutex sync.Mutex
	var handlers sync.WaitGroup
	var clientNumber int
	var serverError error

	activeConnections = make(map[net.Conn]struct{})
	clientNumber = 0
	serverError = nil

	// Close the listener when shutdown is requested.
	go func() {
		<-shutdownContext.Done()
		listener.Close()
	}()
	// Accept incoming connections in a loop until shutdown is requested.
	for {
		var connection net.Conn
		var err error

		connection, err = listener.Accept()

		if err != nil {
			if shutdownContext.Err() != nil {
				break
			}

			serverError = fmt.Errorf("accept TCP connection: %w", err)
			break
		}

		// Do not start a new handler during shutdown.
		if shutdownContext.Err() != nil {
			connection.Close()
			break
		}
		// Assign a unique client ID and start a new goroutine to handle the connection.
		clientNumber++
		var clientID string
		clientID = fmt.Sprintf("client-%03d", clientNumber)

		connectionMutex.Lock()
		activeConnections[connection] = struct{}{}
		connectionMutex.Unlock()

		handlers.Add(1)

		fmt.Printf(
			"Connected %s from %s\n",
			clientID,
			connection.RemoteAddr().String(),
		)

		go handleClient(
			connection,
			clientID,
			shutdownContext,
			activeConnections,
			&connectionMutex,
			&handlers,
		)
	}

	// Close every remaining client connection so blocked handlers
	// can finish their reads and exit.
	connectionMutex.Lock()

	var connection net.Conn
	for connection = range activeConnections {
		connection.Close()
	}

	connectionMutex.Unlock()

	// Wait until every client handler has finished.
	handlers.Wait()

	return serverError
}

// Function: handleClient
// Description:
// handleClient manages the lifecycle of a single client connection.
// It reads data from the connection until the client disconnects or a shutdown signal is received.
// It also ensures that the connection is properly closed and removed from the active connections map.
// Parameters:
// - connection: The net.Conn representing the client connection.
// - clientID: A unique identifier for the client connection.
// - shutdownContext: A context that signals when the server should shut down gracefully.
// - activeConnections: A map of currently active connections, used to track and manage them.
// - connectionMutex: A mutex to synchronize access to the activeConnections map.
// - handlers: A WaitGroup to track the number of active client handlers.
func handleClient(
	connection net.Conn,
	clientID string,
	shutdownContext context.Context,
	activeConnections map[net.Conn]struct{},
	connectionMutex *sync.Mutex,
	handlers *sync.WaitGroup,
) {
	defer handlers.Done()
	// Ensure the connection is removed from the active connections map and closed when the handler exits.
	defer func() {
		connectionMutex.Lock()
		delete(activeConnections, connection)
		connectionMutex.Unlock()

		connection.Close()

		fmt.Printf("Disconnected %s\n", clientID)
	}()

	var bytesReceived int64
	var err error

	bytesReceived, err = io.Copy(io.Discard, connection)
	// If the connection is closed by the client, io.Copy will return an error.
	if err != nil && shutdownContext.Err() == nil {
		fmt.Fprintf(
			os.Stderr,
			"Read error for %s: %v\n",
			clientID,
			err,
		)
	}
	// If the client sent any data before disconnecting, log the number of bytes received.
	if bytesReceived > 0 {
		fmt.Printf(
			"%s disconnected after sending %d raw bytes\n",
			clientID,
			bytesReceived,
		)
	}
}
