package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nikolalohinski/gonja/v2"
)

// setupRoutes walks through the webroot directory and sets up HTTP routes
func setupRoutes(app *App, mux *http.ServeMux) error {
	return filepath.Walk(app.Config.WebRoot, func(path string, info os.FileInfo, err error) error {
		return processFile(app, path, info, err, mux)
	})
}

// processFile handles each file found during directory walk
func processFile(app *App, path string, info os.FileInfo, err error, mux *http.ServeMux) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	// This code definitely has bugs, but it's ok for now
	relativePath := strings.TrimPrefix(path, app.Config.WebRoot)
	file := filepath.Base(relativePath)
	ext := filepath.Ext(file)

	if strings.Contains(file, ".tpl") {
		// Trim the leading slash for display and map key
		displayPath := strings.TrimPrefix(relativePath, "/")
		log.Println("Discovered Template: ", displayPath)
		template, err := gonja.FromFile(path)
		if err != nil {
			log.Printf("Error loading template %s: %v", file, err)
			return nil
		}

		app.tpl[displayPath] = template
		return nil
	}

	if ext != ".sql" {
		return nil
	}

	fileName := strings.TrimSuffix(file, ext)
	dir := filepath.Dir(relativePath)

	secondLevelExt := filepath.Ext(fileName)
	methods := []string{".get", ".post", ".put", ".patch", ".delete", ".options"}

	if secondLevelExt != "" && slices.Contains(methods, secondLevelExt) {
		registerMethodSpecificRoute(app, secondLevelExt, fileName, dir, relativePath, mux)
	} else {
		registerGenericRoute(app, fileName, dir, relativePath, mux)
	}

	// Read the SQL file content and store it in the sqlCache
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Error reading SQL file %s: %v", path, err)
		return err
	}
	// Trim the leading slash for consistency
	cacheKey := strings.TrimPrefix(relativePath, "/")
	app.sqlCache[cacheKey] = string(content)
	log.Println("Loaded route file ", cacheKey)

	app.totalRoutes.Add(1)
	return nil
}

// registerMethodSpecificRoute registers a route for a specific HTTP method
func registerMethodSpecificRoute(app *App, methodExt, fileName, dir, relativePath string, mux *http.ServeMux) {
	effectiveFileName := strings.TrimSuffix(fileName, methodExt)

	if effectiveFileName != "index" {
		dir = filepath.Join(dir, effectiveFileName)
	}

	method := strings.TrimPrefix(strings.ToUpper(methodExt), ".")
	fmt.Printf("%s %s -> %s\n", method, dir, relativePath)
	pathPatterns := extractPathParams(dir)

	routePath := dir
	if !strings.HasSuffix(dir, "/") {
		routePath = fmt.Sprintf("%s/{$}", dir)
	} else {
		routePath = fmt.Sprintf("%s{$}", dir)
	}

	_, err := app.DB.Exec("INSERT INTO wtf_routes (path, method, file) VALUES (?, ?, ?)",
		routePath, method, relativePath)
	if err != nil {
		fmt.Printf("Error inserting into wtf_routes table: %v\n", err)
	}

	mux.HandleFunc(fmt.Sprintf("%s %s", method, routePath), createHandler(app, relativePath, pathPatterns))
}

// registerGenericRoute registers a route that responds to any HTTP method
func registerGenericRoute(app *App, fileName, dir, relativePath string, mux *http.ServeMux) {
	if fileName != "index" {
		dir = filepath.Join(dir, fileName)
	}

	fmt.Printf("ANY %s -> %s\n", dir, relativePath)
	pathPatterns := extractPathParams(dir)

	routePath := dir
	if !strings.HasSuffix(dir, "/") {
		routePath = fmt.Sprintf("%s/{$}", dir)
	} else {
		routePath = fmt.Sprintf("%s{$}", dir)
	}

	_, err := app.DB.Exec("INSERT INTO wtf_routes (path, method, file) VALUES (?, ?, ?)",
		routePath, "ANY", relativePath)
	if err != nil {
		fmt.Printf("Error inserting into wtf_routes table: %v\n", err)
	}

	mux.HandleFunc(routePath, createHandler(app, relativePath, pathPatterns))
}

// extractPathParams extracts path parameters from a URL pattern
// For example, if the pattern is "/users/{id}/profile",
// it will return ["id"]
func extractPathParams(pattern string) []string {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	var params []string

	for _, part := range patternParts {
		// Check if this part is a parameter (enclosed in {})
		if len(part) > 2 && part[0] == '{' && part[len(part)-1] == '}' {
			// Extract the parameter name without the braces
			paramName := part[1 : len(part)-1]
			params = append(params, paramName)
		}
	}

	return params
}
