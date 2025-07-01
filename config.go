package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	// need to migrate most of this to Credential struct
	ENV                string // DEV, Prod, Local, Hosted
	TopLevelDir        string // Top level directory of the application.
	UserSettingsDB     string // Application Support App settings like store of credentials, known connections.
	ServiceAccountJson string // Need to move ServiceAccountJson to credential struct.
	Port               string // Config.Port is the port that Mavgo Flight service binds to.  Do not confuse with port of a request.
	TopCacheDir        string // Remote files and local files cached as sqlite land in this folder
	DefaultFormat      string // I have no idea.  Need to track where this is used.
	ServeFolder        string // I supersetted/wrapped/inherited http.FileServer as starting point of FlightHandler. ServeFolder is the folder it starts for serving.
	AllowStaging       bool   // Flag to enable staging files as Sqlite.
	PrivateKeyPath     string // Need to move PrivateKeyPath to Credential struct.
	ProjectID          string // Until I create a better solution assuming that Mavgo Flight is serving data from services tied to one single Google Cloud project 	// I created this variable to enable NewClient for bigquery July 27 2024.
}

var (
	C Config
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
	C = Config{}
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
