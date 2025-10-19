package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/sad-pixel/wtfhttpd/cache"
	"github.com/sad-pixel/wtfhttpd/udfs"
)

var logo = "\n" +
	"               .oOo  o                           o \n" +
	"               O    O                           O  \n" +
	"           O   o    o       O     O             o  \n" +
	"          oOo  OoO  O      oOo   oOo            o  \n" +
	"'o     O   o   o    OoOo.   o     o   .oOo. .oOoO  \n" +
	" O  o  o   O   O    o   o   O     O   O   o o   O  \n" +
	" o  O  O   o   o    o   O   o     o   o   O O   o  \n" +
	" `Oo'oO'   `oO O'   O   o   `oO   `oO oOoO' `OoO'o \n" +
	"                                      O            \n" +
	"                                      o'           \n"

func main() {
	fmt.Println(logo)
	kvCache := cache.NewKVCache()
	gonja.DefaultConfig.AutoEscape = true
	udfs.RegisterUdfs(kvCache)

	config := LoadConfig()

	db, err := sql.Open("sqlite", config.Db)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	// Create the wtf_routes table to track routes
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS wtf_routes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			file TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatalf("Error creating wtf_routes table: %v", err)
	}

	if config.LoadDotenv {
		log.Println("Loading .env file from webroot/.env")
		err := godotenv.Load(fmt.Sprintf("%s/.env", config.WebRoot))
		if err != nil {
			log.Printf("Warning: Could not load .env file: %v", err)
		}
	}

	app := &App{
		Config:    config,
		DB:        db,
		startedAt: time.Now(),
		kv:        kvCache,
		vd:        validator.New(),
	}

	if err := app.reloadRoutes(); err != nil {
		return
	}

	if config.LiveReload {
		log.Println("Starting Live Reloader")
		go app.liveReloader()
	}

	listen := fmt.Sprintf("%s:%d", config.Host, config.Port)

	log.Println("Server starting on ", listen)
	http.ListenAndServe(listen, app)
}
