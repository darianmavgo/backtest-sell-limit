package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/darianmavgo/backtest-sell-limit/pkg/types"
)

var (
	C types.Config
)

func InitConfig() {
	log.Println("Starting initConfig")
	cfile, err := findConfigFile()
	if err != nil {
		log.Fatalln("Could not load config.json ", err)

	}
	file, err := os.Open(cfile)
	if err != nil {
		log.Fatalln("Could not load config.json ", err)
	}

	decoder := json.NewDecoder(file)
	C = types.Config{}
	err = decoder.Decode(&C)
	if err != nil {
		log.Println(err, "initConfig")
	}
}

// ConfigSearchPaths returns a list of paths to search for config files in order of preference
func ConfigSearchPaths() []string {
	// Get executable path
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	exeDir := filepath.Dir(exe)

	// Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	// Search paths in order of preference
	paths := []string{
		filepath.Join(workDir, "config.json"),            // Current directory
		filepath.Join(workDir, "config", "config.json"),  // ./config/
		filepath.Join(exeDir, "config.json"),             // Executable directory
		filepath.Join(exeDir, "config", "config.json"),   // Executable's config/
		filepath.Join(homeDir, ".flight", "config.json"), // ~/.flight/
		"/etc/flight/config.json",                        // System-wide
	}

	return paths
}

// findConfigFile searches for a config file in standard locations
func findConfigFile() (string, error) {
	// First check if --config flag was specified
	if len(os.Args) > 1 {
		for i, arg := range os.Args {
			if arg == "--config" && i+1 < len(os.Args) {
				configPath := os.Args[i+1]
				if _, err := os.Stat(configPath); err == nil {
					return configPath, nil
				}
				return "", fmt.Errorf("config file not found at specified path: %s", configPath)
			}
		}
	}

	paths := ConfigSearchPaths()

	// Try each path
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// If no config file found, return error
	return "", fmt.Errorf("no config file found in search paths: %v", paths)
}
