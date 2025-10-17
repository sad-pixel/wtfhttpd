package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// App holds application-wide dependencies
type App struct {
	Config        *Config
	DB            *sql.DB
	startedAt     time.Time
	hitsProcessed atomic.Int64
	totalRoutes   atomic.Int64

	mu     sync.RWMutex
	router http.Handler
}

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.mu.RLock()
	router := app.router
	app.mu.RUnlock()

	router.ServeHTTP(w, r)
}

func (app *App) reloadRoutes() error {
	log.Println("Reloading Routes...")

	// Clear the wtf_routes table before reloading
	_, err := app.DB.Exec("DELETE FROM wtf_routes")
	if err != nil {
		log.Printf("Error truncating wtf_routes table: %v", err)
		return err
	}

	app.totalRoutes.Store(0)

	mux := http.NewServeMux()
	mux.HandleFunc("/_wtf", app.serveAdmin)

	err = setupRoutes(app, mux)
	if err != nil {
		log.Printf("Error during route reload: %v", err)
		return err
	}

	app.mu.Lock()
	app.router = mux
	app.mu.Unlock()

	log.Printf("Total routes: %d", app.totalRoutes.Load())
	return nil
}

func (app *App) liveReloader() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Initial setup of watchers for all existing directories
	if err := filepath.Walk(app.Config.WebRoot, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	}); err != nil {
		log.Fatalf("Failed to set up directory watch: %v", err)
	}

	var debounceTimer *time.Timer

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Check if a directory was created
				if event.Has(fsnotify.Create) {
					fileInfo, err := os.Stat(event.Name)
					if err == nil && fileInfo.IsDir() {
						log.Printf("New directory created: %s, adding to watcher", event.Name)
						watcher.Add(event.Name)
					}
				}

				if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) || event.Has(fsnotify.Write) {
					if debounceTimer != nil {
						debounceTimer.Reset(200 * time.Millisecond)
					} else {
						debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
							// If directories were added or removed, we need to refresh our watchers
							if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
								// Re-scan all directories to ensure we're watching everything
								filepath.Walk(app.Config.WebRoot, func(path string, info os.FileInfo, err error) error {
									if err == nil && info != nil && info.IsDir() {
										watcher.Add(path)
									}
									return nil
								})
							}

							if err := app.reloadRoutes(); err != nil {
								log.Printf("Error applying reloaded routes: %v", err)
							}
						})
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	<-make(chan struct{})
}
