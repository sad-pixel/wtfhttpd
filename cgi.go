package main

import (
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
)

// createCGIHandler returns a handler function for CGI scripts
// Leaves out error handling for clarity
func CreateCGIHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		effectivePath := filepath.Join("./webroot", path)

		// Make executable if it's not already
		if !isExecutable(effectivePath) {
			makeExecutable(effectivePath)
		}

		// Execute the CGI script
		cmd := exec.Command(effectivePath)

		// Serve the CGI script
		handler := cgi.Handler{Path: cmd.Path, Dir: cmd.Dir, Env: cmd.Env}
		handler.ServeHTTP(w, r)
	}
}

// Check if a file is executable
func isExecutable(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	// Check if the file has execute permission
	return info.Mode()&0111 != 0
}

// Make a file executable
func makeExecutable(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		log.Printf("Error getting file info for %s: %v", filename, err)
		return false
	}

	// Add execute permission (user, group, others)
	err = os.Chmod(filename, info.Mode()|0111)
	if err != nil {
		log.Printf("Error making %s executable: %v", filename, err)
		return false
	}

	return true
}
