package config

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
)

// Config holds the configuration values for the Retention Agent Platform
type Config struct {
	DatabaseURL  string `json:"database_url"`
	RedisURL     string `json:"redis_url"`
	Port         string `json:"port"`
	GeminiAPIKey string `json:"gemini_api_key"`
	LLMModel     string `json:"llm_model"`
	Role         string `json:"role"`
}

// AppConfig is the globally available configuration settings
var AppConfig Config

// Load loads the configuration from multiple sources with defined precedence:
// 1. Defaults
// 2. .env file
// 3. config.json file
// 4. OS environment variables (highest priority)
func Load() {
	// 1. Set default values
	AppConfig = Config{
		DatabaseURL:  "postgres://postgres:postgres@db:5432/retention?sslmode=disable",
		RedisURL:     "redis:6379",
		Port:         "8080",
		LLMModel:     "gemini-1.5-flash",
		Role:         "both",
	}

	// 2. Load from .env file if it exists, setting OS environment variables if not already set
	loadEnvFile(".env")

	// 3. Load from config.json if it exists
	loadJSONFile("config.json")

	// 4. Override with OS environment variables if set (highest precedence)
	overrideWithEnv()

	// Ensure essential variables are set as environment variables as well for any sub-libraries
	os.Setenv("DATABASE_URL", AppConfig.DatabaseURL)
	os.Setenv("REDIS_URL", AppConfig.RedisURL)
	os.Setenv("PORT", AppConfig.Port)
	os.Setenv("GEMINI_API_KEY", AppConfig.GeminiAPIKey)
	os.Setenv("LLM_MODEL", AppConfig.LLMModel)
	os.Setenv("ROLE", AppConfig.Role)

	// Log configuration metadata safely (redacting secret)
	apiKeyStatus := "not configured"
	if AppConfig.GeminiAPIKey != "" {
		apiKeyStatus = "configured (redacted)"
	}
	log.Printf("[Config] Loaded: DatabaseURL=%s, RedisURL=%s, Port=%s, LLMModel=%s, Role=%s, GeminiAPIKey=%s",
		AppConfig.DatabaseURL, AppConfig.RedisURL, AppConfig.Port, AppConfig.LLMModel, AppConfig.Role, apiKeyStatus)
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // Ignore if file doesn't exist
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func loadJSONFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return // Ignore if file doesn't exist
	}
	defer file.Close()

	var fileConfig Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&fileConfig); err != nil {
		log.Printf("[Config] Warning: Failed to decode JSON file %s: %v", path, err)
		return
	}

	if fileConfig.DatabaseURL != "" {
		AppConfig.DatabaseURL = fileConfig.DatabaseURL
	}
	if fileConfig.RedisURL != "" {
		AppConfig.RedisURL = fileConfig.RedisURL
	}
	if fileConfig.Port != "" {
		AppConfig.Port = fileConfig.Port
	}
	if fileConfig.GeminiAPIKey != "" {
		AppConfig.GeminiAPIKey = fileConfig.GeminiAPIKey
	}
	if fileConfig.LLMModel != "" {
		AppConfig.LLMModel = fileConfig.LLMModel
	}
	if fileConfig.Role != "" {
		AppConfig.Role = fileConfig.Role
	}
}

func overrideWithEnv() {
	if val := os.Getenv("DATABASE_URL"); val != "" {
		AppConfig.DatabaseURL = val
	}
	if val := os.Getenv("REDIS_URL"); val != "" {
		AppConfig.RedisURL = val
	}
	if val := os.Getenv("PORT"); val != "" {
		AppConfig.Port = val
	}
	if val := os.Getenv("GEMINI_API_KEY"); val != "" {
		AppConfig.GeminiAPIKey = val
	}
	if val := os.Getenv("LLM_MODEL"); val != "" {
		AppConfig.LLMModel = val
	}
	if val := os.Getenv("ROLE"); val != "" {
		AppConfig.Role = val
	}
}
