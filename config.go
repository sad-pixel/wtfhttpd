package main

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Host          string `toml:"host"`
	Port          int    `toml:"port"`
	Db            string `toml:"db"`
	WebRoot       string `toml:"web_root"`
	LiveReload    bool   `toml:"live_reload"`
	EnableAdmin   bool   `toml:"enable_admin"`
	AdminUsername string `toml:"admin_username"`
	AdminPassword string `toml:"admin_password"`
}

func NewConfig() *Config {
	return &Config{
		Host:          "127.0.0.1",
		Port:          8080,
		Db:            "wtf.db",
		WebRoot:       "webroot",
		LiveReload:    true,
		EnableAdmin:   true,
		AdminUsername: "wtfhttpd",
		AdminPassword: "wtfhttpd",
	}
}

func LoadConfig() *Config {
	config := NewConfig()

	if _, err := toml.DecodeFile("wtf.toml", config); err != nil {
		// If the file doesn't exist, that's okay! We just log it
		// and continue with the default values.
		if !os.IsNotExist(err) {
			// If it's some other error (e.g., malformed TOML), we should probably exit.
			log.Fatalf("Error reading config file wtf.toml: %v", err)
		}
		log.Println("wtf.toml not found, using default configuration.")
	} else {
		log.Println("Loaded configuration from wtf.toml.")
	}

	return config
}
