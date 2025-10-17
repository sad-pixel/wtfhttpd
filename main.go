package main

import (
	"database/sql"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/nikolalohinski/gonja/v2"
)

// App holds application-wide dependencies
type App struct {
	DB            *sql.DB
	startedAt     time.Time
	hitsProcessed atomic.Int64
	totalRoutes   atomic.Int64
}

func main() {
	gonja.DefaultConfig.AutoEscape = true
	registerUdfs()

	db, err := sql.Open("sqlite", "./wtf.db")
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	app := &App{
		DB:        db,
		startedAt: time.Now(),
	}

	if err := setupRoutes(app); err != nil {
		log.Println(err)
	}

	log.Println("Server starting on localhost:8080")
	// Add a handler for the _wtf endpoint to show server stats
	http.HandleFunc("/_wtf", app.serveAdmin)
	http.ListenAndServe("localhost:8080", nil)
}
