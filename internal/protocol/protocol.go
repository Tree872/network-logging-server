// File: protocol.go
// Description: This file contains the protocol definitions and functions for parsing requests and writing responses.
// It defines the Request and Response structures, as well as functions to parse incoming requests and write responses in JSON format.
package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Level string

const (
	LevelEmergency Level = "EMERGENCY"
	LevelAlert     Level = "ALERT"
	LevelCritical  Level = "CRITICAL"
	LevelError     Level = "ERROR"
	LevelWarning   Level = "WARNING"
	LevelNotice    Level = "NOTICE"
	LevelInfo      Level = "INFO"
	LevelDebug     Level = "DEBUG"
)

const (
	StatusOK              = "OK"
	StatusInvalidMessage  = "INVALID_MESSAGE"
	StatusMessageTooLarge = "MESSAGE_TOO_LARGE"
	StatusInternalError   = "INTERNAL_ERROR"
	StatusRateLimited     = "RATE_LIMITED"
)

// Function: IsValid
// Description:
// IsValid checks if the given Level is valid.
// Parameters:
// - level: The Level to check.
// Returns:
// - bool: True if the level is valid, false otherwise.
func (level Level) IsValid() bool {
	switch level {
	case LevelEmergency,
		LevelAlert,
		LevelCritical,
		LevelError,
		LevelWarning,
		LevelNotice,
		LevelInfo,
		LevelDebug:
		return true
	default:
		return false
	}
}

type Request struct {
	Level     Level  `json:"level"`
	Service   string `json:"service"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

type Response struct {
	Status   string `json:"status"`
	ClientID string `json:"clientId,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Function: ParseRequest
// Description:
// ParseRequest takes a byte slice containing JSON data and attempts to decode it into a Request struct.
// It returns the parsed Request and an error if the parsing fails or if the request is invalid.
// Parameters:
// - data: A byte slice containing the JSON-encoded request data.
// Returns:
// - Request: The parsed Request struct.
// - error: An error if the parsing fails or if the request is invalid.
func ParseRequest(data []byte) (Request, error) {
	var request Request

	var decoder *json.Decoder
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var err error
	err = decoder.Decode(&request)
	if err != nil {
		return request, fmt.Errorf("invalid JSON request: %w", err)
	}

	var extraValue interface{}
	err = decoder.Decode(&extraValue)

	if err != io.EOF {
		if err == nil {
			return request, fmt.Errorf("request contains multiple JSON values")
		}

		return request, fmt.Errorf("invalid data after JSON request: %w", err)
	}

	if !request.Level.IsValid() {
		return request, fmt.Errorf("unsupported severity level")
	}

	if strings.TrimSpace(request.Service) == "" {
		return request, fmt.Errorf("service is required")
	}

	if strings.TrimSpace(request.Message) == "" {
		return request, fmt.Errorf("message is required")
	}

	if strings.TrimSpace(request.RequestID) != request.RequestID {
		return request, fmt.Errorf("requestId cannot have surrounding whitespace")
	}

	return request, nil
}

// Function: WriteResponse
// Description:
// WriteResponse encodes the given Response struct as JSON and writes it to the provided io.Writer.
// It returns an error if the encoding or writing process fails.
// Parameters:
// - writer: The io.Writer to which the JSON-encoded response will be written.
// - response: The Response struct to be encoded and written.
// Returns:
// - error: An error if the encoding or writing process fails.
func WriteResponse(writer io.Writer, response Response) error {
	var encoder *json.Encoder
	encoder = json.NewEncoder(writer)

	var err error
	err = encoder.Encode(response)
	if err != nil {
		return fmt.Errorf("write protocol response: %w", err)
	}

	return nil
}
