// File: config.go
// Description: This file contains the configuration loading and validation logic for the network logging server.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const MaxProtocolMessageSize int = 65536

type Config struct {
	Host                   string
	Port                   int
	LogFile                string
	LogFormat              string
	MinimumLevel           string
	MaxMessageSize         int
	RateLimitMessages      int
	RateLimitWindowSeconds int
}

// Function: Load
// Description:
// Load the configuration from a file and return a Config struct.
// Validate format errors, missing required settings, and unknown settings.
// Parameters:
// - configPath: The path to the configuration file.
// Returns:
// - Config: The loaded configuration.
// - error: An error if the configuration could not be loaded or validated.
func Load(configPath string) (Config, error) {
	var config Config
	var absoluteConfigPath string
	var err error

	absoluteConfigPath, err = filepath.Abs(configPath)
	if err != nil {
		return config, fmt.Errorf("resolve configuration path: %w", err)
	}

	var configFile *os.File
	configFile, err = os.Open(absoluteConfigPath)
	if err != nil {
		return config, fmt.Errorf("open configuration file %q: %w", absoluteConfigPath, err)
	}
	defer configFile.Close()

	var values map[string]string = make(map[string]string)

	var scanner *bufio.Scanner = bufio.NewScanner(configFile)
	var lineNumber int = 0

	for scanner.Scan() {
		lineNumber++

		var line string = strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var key string
		var value string
		var found bool
		key, value, found = strings.Cut(line, "=")
		if !found {
			return config, fmt.Errorf(
				"configuration line %d: expected key=value",
				lineNumber,
			)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if key == "" {
			return config, fmt.Errorf(
				"configuration line %d: setting name cannot be empty",
				lineNumber,
			)
		}

		if value == "" {
			return config, fmt.Errorf(
				"configuration line %d: value for %q cannot be empty",
				lineNumber,
				key,
			)
		}

		var exists bool
		_, exists = values[key]
		if exists {
			return config, fmt.Errorf(
				"configuration line %d: duplicate setting %q",
				lineNumber,
				key,
			)
		}

		values[key] = value
	}

	var scanErr error
	if scanErr = scanner.Err(); scanErr != nil {
		return config, fmt.Errorf("read configuration file: %w", scanErr)
	}

	var allowedKeys map[string]bool = map[string]bool{
		"host":                      true,
		"port":                      true,
		"log_file":                  true,
		"log_format":                true,
		"minimum_level":             true,
		"max_message_size":          true,
		"rate_limit_messages":       true,
		"rate_limit_window_seconds": true,
	}

	var key string
	for key = range values {
		if !allowedKeys[key] {
			return config, fmt.Errorf("unknown configuration setting %q", key)
		}
	}

	var requiredKeys []string = []string{
		"host",
		"port",
		"log_file",
		"log_format",
		"minimum_level",
		"max_message_size",
		"rate_limit_messages",
		"rate_limit_window_seconds",
	}

	for _, key = range requiredKeys {
		var exists bool
		_, exists = values[key]
		if !exists {
			return config, fmt.Errorf(
				"missing required configuration setting %q",
				key,
			)
		}
	}

	config.Host = values["host"]
	config.LogFile = values["log_file"]
	config.LogFormat = values["log_format"]
	config.MinimumLevel = values["minimum_level"]

	config.Port, err = strconv.Atoi(values["port"])
	if err != nil {
		return config, fmt.Errorf("port must be an integer: %w", err)
	}

	config.MaxMessageSize, err = strconv.Atoi(values["max_message_size"])
	if err != nil {
		return config, fmt.Errorf("max_message_size must be an integer: %w", err)
	}

	config.RateLimitMessages, err = strconv.Atoi(values["rate_limit_messages"])
	if err != nil {
		return config, fmt.Errorf("rate_limit_messages must be an integer: %w", err)
	}

	config.RateLimitWindowSeconds, err = strconv.Atoi(
		values["rate_limit_window_seconds"],
	)
	if err != nil {
		return config, fmt.Errorf(
			"rate_limit_window_seconds must be an integer: %w",
			err,
		)
	}

	var validationErr error
	if validationErr = config.Validate(); validationErr != nil {
		return config, validationErr
	}

	var configDirectory string = filepath.Dir(absoluteConfigPath)

	if !filepath.IsAbs(config.LogFile) {
		config.LogFile = filepath.Join(configDirectory, config.LogFile)
	}

	config.LogFile = filepath.Clean(config.LogFile)

	return config, nil
}

// Fuction: Validate
// Description: Validate the configuration settings and return an error if any setting is invalid.
//
//	Validate logical constraints, such as valid port range, supported log formats, and severity levels.
//
// Parameters:
// - config: The Config struct to validate.
// Returns:
// - error: An error if any configuration setting is invalid.
func (config Config) Validate() error {
	if strings.TrimSpace(config.Host) == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if config.LogFile == "" {
		return fmt.Errorf("log_file cannot be empty")
	}

	if config.LogFormat != "text" && config.LogFormat != "jsonl" {
		return fmt.Errorf("log_format must be text or jsonl")
	}

	var validLevels map[string]bool = map[string]bool{
		"EMERGENCY": true,
		"ALERT":     true,
		"CRITICAL":  true,
		"ERROR":     true,
		"WARNING":   true,
		"NOTICE":    true,
		"INFO":      true,
		"DEBUG":     true,
	}

	if !validLevels[config.MinimumLevel] {
		return fmt.Errorf("minimum_level is not a supported severity")
	}

	if config.MaxMessageSize < 1 || config.MaxMessageSize > MaxProtocolMessageSize {
		return fmt.Errorf(
			"max_message_size must be between 1 and %d",
			MaxProtocolMessageSize,
		)
	}

	if config.RateLimitMessages < 1 {
		return fmt.Errorf("rate_limit_messages must be at least 1")
	}

	if config.RateLimitWindowSeconds < 1 {
		return fmt.Errorf("rate_limit_window_seconds must be at least 1")
	}

	return nil
}
