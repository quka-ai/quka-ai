package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

// LoadEnv loads the .env file from the project root directory
func LoadEnv() error {
	// Get the current file's directory
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)

	// Navigate to project root (go up from pkg/testutils)
	projectRoot := filepath.Join(dir, "..", "..")
	envPath := filepath.Join(projectRoot, ".env")
	fmt.Println(envPath)
	// Check if .env file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// .env file doesn't exist, continue without error
		return nil
	}

	// Load .env file
	return godotenv.Load(envPath)
}

// LoadEnvOrPanic loads the .env file and panics if there's an error
func LoadEnvOrPanic() {
	if err := LoadEnv(); err != nil {
		panic("Failed to load .env file: " + err.Error())
	}
}

// GetEnvOrDefault gets an environment variable with a default value
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
