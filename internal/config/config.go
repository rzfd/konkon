package config

import (
	"os"
)

// Config holds runtime settings for the konkon server.
type Config struct {
	ListenAddr string
	DataDir    string
	DBPath     string
	UploadDir  string
}

// Load reads configuration from environment variables with defaults.
func Load() Config {
	c := Config{
		ListenAddr: ":8080",
		DataDir:    "./data",
	}
	if v := os.Getenv("KONKON_LISTEN"); v != "" {
		c.ListenAddr = v
	}
	if v := os.Getenv("KONKON_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	c.DBPath = c.DataDir + "/konkon.db"
	c.UploadDir = c.DataDir + "/uploads"
	return c
}
