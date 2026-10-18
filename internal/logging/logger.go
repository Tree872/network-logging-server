// File: internal/logging/logger.go
// Description: This file contains the Logger implementation for the network logging server.
// It provides functionality to write log entries to a file in either text or JSONL format, ensuring thread-safe access to the log file.
// Author: Tri Phan
// SENG2045_A2
package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"network-logging-server/internal/protocol"
)

type Logger struct {
	file   *os.File
	format string
	mutex  sync.Mutex
}

type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	ClientID  string         `json:"clientId"`
	Level     protocol.Level `json:"level"`
	Service   string         `json:"service"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
}

// Function: New
// Description:
// New creates a new Logger instance that writes log entries to the specified file path in the given format.
// It ensures that the log directory exists and opens the log file for appending.
// Parameters:
// - path: The file path where log entries will be written.
// - format: The format of the log entries, either "text" or "jsonl".
// Returns:
// - *Logger: A pointer to the created Logger instance.
// - error: An error if the logger could not be created, such as if the log directory could not be created or the log file could not be opened.
func New(path string, format string) (*Logger, error) {
	var logger Logger
	var err error

	if format != "text" && format != "jsonl" {
		return nil, fmt.Errorf("unsupported log format %q", format)
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	logger.file, err = os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	logger.format = format

	return &logger, nil
}

// Function: Write
// Description:
// Write writes a log entry to the log file in the specified format (text or JSONL).
// It ensures thread-safe access to the log file using a mutex.
// Parameters:
// - clientID: The unique identifier of the client that sent the log message.
// - request: The protocol.Request containing the log message details.
// Returns:
// - error: An error if the log entry could not be written, such as if the write operation failed or was incomplete.
func (logger *Logger) Write(clientID string, request protocol.Request) error {
	var entry LogEntry
	var data []byte
	var err error

	entry.Timestamp = time.Now().UTC()
	entry.ClientID = clientID
	entry.Level = request.Level
	entry.Service = request.Service
	entry.Message = request.Message
	entry.RequestID = request.RequestID

	// If the format is JSONL, encode the log entry as JSON and append a newline.
	if logger.format == "jsonl" {
		var buffer bytes.Buffer
		var encoder *json.Encoder

		encoder = json.NewEncoder(&buffer)
		err = encoder.Encode(entry)
		if err != nil {
			return fmt.Errorf("format JSON log entry: %w", err)
		}

		data = buffer.Bytes()
		// If the format is text, construct a log entry string with key-value pairs and a newline.
	} else {
		data = []byte(
			"timestamp=" + entry.Timestamp.Format(time.RFC3339Nano) +
				" level=" + string(entry.Level) +
				" clientId=" + strconv.Quote(entry.ClientID) +
				" service=" + strconv.Quote(entry.Service) +
				" message=" + strconv.Quote(entry.Message) +
				formatRequestID(entry.RequestID) +
				"\n",
		)
	}

	logger.mutex.Lock()
	defer logger.mutex.Unlock()

	var bytesWritten int
	bytesWritten, err = logger.file.Write(data)
	if err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	if bytesWritten != len(data) {
		return fmt.Errorf("write log entry: incomplete write")
	}

	return nil
}

// Function: formatRequestID
// Description:
// formatRequestID formats the request ID for inclusion in a log entry.
// If the request ID is empty, it returns an empty string. Otherwise, it returns a formatted string with the request ID.
// Parameters:
// - requestID: The request ID to format.
// Returns:
// - string: The formatted request ID string, or an empty string if the request ID is empty.
func formatRequestID(requestID string) string {
	if requestID == "" {
		return ""
	}

	return " requestId=" + strconv.Quote(requestID)
}

// Function: Close
// Description:
// Close closes the log file associated with the Logger instance.
// It ensures thread-safe access to the log file using a mutex.
// Returns:
// - error: An error if the log file could not be closed.
func (logger *Logger) Close() error {
	logger.mutex.Lock()
	defer logger.mutex.Unlock()

	var err error
	err = logger.file.Close()
	if err != nil {
		return fmt.Errorf("close log file: %w", err)
	}

	return nil
}
