// File: server.go
// Description: This file contains the server logic for the network logging server application.
// It handles incoming TCP connections, manages client handlers, and ensures graceful shutdown.
package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"network-logging-server/internal/logging"
	"network-logging-server/internal/protocol"
	"network-logging-server/internal/ratelimit"
)

// Function: Run
// Description:
// Run starts the server to accept incoming TCP connections and handle them concurrently.
// It listens for incoming connections on the provided listener and spawns a new goroutine for each connection.
// The server continues to accept connections until a shutdown signal is received via the shutdownContext.
// Parameters:
// - listener: The net.Listener to accept incoming TCP connections.
// - shutdownContext: A context that signals when the server should shut down gracefully.
// - maxMessageSize: The maximum allowed size for incoming messages from clients.
// - logWriter: A pointer to the Logger instance used to write log entries.
// - rateLimitMessages: The maximum number of messages allowed per client within the rate limit window.
// - rateLimitWindowSeconds: The duration of the rate limit window in seconds.
// Returns:
// - error: An error if the server encounters an issue while running.
func Run(
	listener net.Listener,
	shutdownContext context.Context,
	maxMessageSize int,
	logWriter *logging.Logger,
	rateLimitMessages int,
	rateLimitWindowSeconds int,
) error {
	var activeConnections map[net.Conn]struct{}
	var connectionMutex sync.Mutex
	var handlers sync.WaitGroup
	var clientNumber int
	var serverError error
	var limiter *ratelimit.Limiter

	activeConnections = make(map[net.Conn]struct{})
	clientNumber = 0
	serverError = nil
	limiter = ratelimit.New(
		rateLimitMessages,
		time.Duration(rateLimitWindowSeconds)*time.Second,
	)

	go func() {
		<-shutdownContext.Done()
		listener.Close()
	}()

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

		if shutdownContext.Err() != nil {
			connection.Close()
			break
		}

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
			maxMessageSize,
			logWriter,
			limiter,
			activeConnections,
			&connectionMutex,
			&handlers,
		)
	}

	connectionMutex.Lock()

	var connection net.Conn
	for connection = range activeConnections {
		connection.Close()
	}

	connectionMutex.Unlock()

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
// - maxMessageSize: The maximum allowed size for incoming messages from the client.
// - logWriter: A pointer to the Logger instance used to write log entries.
// - limiter: A pointer to the Limiter instance used to enforce rate limiting for incoming requests.
// - activeConnections: A map of currently active connections, used to track and manage them.
// - connectionMutex: A mutex to synchronize access to the activeConnections map.
// - handlers: A WaitGroup to track the number of active client handlers.
func handleClient(
	connection net.Conn,
	clientID string,
	shutdownContext context.Context,
	maxMessageSize int,
	logWriter *logging.Logger,
	limiter *ratelimit.Limiter,
	activeConnections map[net.Conn]struct{},
	connectionMutex *sync.Mutex,
	handlers *sync.WaitGroup,
) {
	defer handlers.Done()

	defer func() {
		connectionMutex.Lock()
		delete(activeConnections, connection)
		connectionMutex.Unlock()

		connection.Close()
		fmt.Printf("Disconnected %s\n", clientID)
	}()

	var clientIP string
	var splitErr error

	clientIP, _, splitErr = net.SplitHostPort(
		connection.RemoteAddr().String(),
	)
	if splitErr != nil {
		clientIP = connection.RemoteAddr().String()
	}

	var reader *bufio.Reader
	reader = bufio.NewReaderSize(connection, 4096)

	for {
		var line []byte
		var tooLarge bool
		var err error

		line, tooLarge, err = readRequestLine(reader, maxMessageSize)
		if err != nil {
			if !errors.Is(err, io.EOF) && shutdownContext.Err() == nil {
				fmt.Fprintf(
					os.Stderr,
					"Read error for %s: %v\n",
					clientID,
					err,
				)
			}
			return
		}

		var response protocol.Response
		response.ClientID = clientID

		if !limiter.Allow(clientIP) {
			response.Status = protocol.StatusRateLimited
			response.Error = "Rate limit exceeded; try again after the configured window"
		} else if tooLarge {
			response.Status = protocol.StatusMessageTooLarge
			response.Error = "Request exceeds maximum size"
		} else {
			var request protocol.Request

			request, err = protocol.ParseRequest(line)
			if err != nil {
				response.Status = protocol.StatusInvalidMessage
				response.Error = err.Error()
			} else {
				err = logWriter.Write(clientID, request)
				if err != nil {
					fmt.Fprintf(
						os.Stderr,
						"Log write error for %s: %v\n",
						clientID,
						err,
					)

					response.Status = protocol.StatusInternalError
					response.Error = "Could not write log entry"
				} else {
					response.Status = protocol.StatusOK

					fmt.Printf(
						"Logged message from %s: level=%s service=%s\n",
						clientID,
						request.Level,
						request.Service,
					)
				}
			}
		}

		err = protocol.WriteResponse(connection, response)
		if err != nil {
			if shutdownContext.Err() == nil {
				fmt.Fprintf(
					os.Stderr,
					"Response error for %s: %v\n",
					clientID,
					err,
				)
			}
			return
		}
	}
}

// Function: readRequestLine
// Description:
// readRequestLine reads a single line of input from the provided bufio.Reader.
// It enforces a maximum message size and returns an error if the line exceeds this size.
// The function also handles end-of-file (EOF) and unexpected EOF conditions.
// Parameters:
// - reader: A bufio.Reader to read input from.
// - maxMessageSize: The maximum allowed size for the input line.
// Returns:
// - []byte: The read line of input, excluding the newline character.
// - bool: A flag indicating whether the line exceeded the maximum size.
// - error: An error if reading fails or if EOF is encountered unexpectedly.
func readRequestLine(
	reader *bufio.Reader,
	maxMessageSize int,
) ([]byte, bool, error) {
	var line []byte
	var totalBytes int
	var tooLarge bool

	line = make([]byte, 0, maxMessageSize)

	for {
		var currentByte byte
		var err error

		currentByte, err = reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if totalBytes == 0 {
					return nil, false, io.EOF
				}
				return nil, false, io.ErrUnexpectedEOF
			}
			return nil, false, err
		}

		totalBytes++

		if totalBytes > maxMessageSize {
			tooLarge = true
		} else {
			line = append(line, currentByte)
		}

		if currentByte == '\n' {
			if tooLarge {
				return nil, true, nil
			}

			line = line[:len(line)-1]

			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}

			return line, false, nil
		}
	}
}
