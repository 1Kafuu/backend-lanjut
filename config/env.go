package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func LoadENV() {
	if err:= godotenv.Load(); err != nil {
		log.Println("Peringatan: berkas .env tidak ditemukan, memakai environment sistem")
	}
}

func GetENV(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func GetENVInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Peringatan: %s bukan (%q), memakai bawaan %d", key, value, fallback)
		return fallback
	}
	return parsed
}