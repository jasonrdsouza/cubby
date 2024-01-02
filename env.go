package main

import (
	"bufio"
	"os"
	"strings"
)

// SetENV reads environment variables from a .env file and sets them as
// environment variables in the current context.
func SetENV() error {
	return SetEnvFromFile(".env")
}

// SetEnvFromFile reads environment variables from the specified file and sets
// them in the current context.
func SetEnvFromFile(envFilePath string) error {
	envFile, err := os.Open(envFilePath)
	if err != nil {
		return err
	}
	defer envFile.Close()

	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
