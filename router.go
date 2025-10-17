package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// setupRoutes walks through the webroot directory and sets up HTTP routes
func setupRoutes(app *App) error {
	return filepath.Walk("./webroot", func(path string, info os.FileInfo, err error) error {
		return processFile(app, path, info, err)
	})
}

// processFile handles each file found during directory walk
func processFile(app *App, path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	// This code definitely has bugs, but it's ok for now
	relativePath := strings.TrimPrefix(path, "webroot")
	file := filepath.Base(relativePath)
	ext := filepath.Ext(file)

	if ext != ".sql" {
		return nil
	}

	fileName := strings.TrimSuffix(file, ext)
	dir := filepath.Dir(relativePath)

	secondLevelExt := filepath.Ext(fileName)
	methods := []string{".get", ".post", ".put", ".patch", ".delete", ".options"}

	if secondLevelExt != "" && slices.Contains(methods, secondLevelExt) {
		registerMethodSpecificRoute(app, secondLevelExt, fileName, dir, relativePath)
	} else {
		registerGenericRoute(app, fileName, dir, relativePath)
	}
	app.totalRoutes.Add(1)

	return nil
}

// registerMethodSpecificRoute registers a route for a specific HTTP method
func registerMethodSpecificRoute(app *App, methodExt, fileName, dir, relativePath string) {
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

	http.HandleFunc(fmt.Sprintf("%s %s", method, routePath), createHandler(app, relativePath, pathPatterns))
}

// registerGenericRoute registers a route that responds to any HTTP method
func registerGenericRoute(app *App, fileName, dir, relativePath string) {
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

	http.HandleFunc(routePath, createHandler(app, relativePath, pathPatterns))
}
