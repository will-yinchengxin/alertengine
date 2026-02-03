package config

import "fmt"

type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("config error: %s", e.Message)
}

func ErrInvalidConfig(msg string) error {
	return ConfigError{Message: msg}
}
