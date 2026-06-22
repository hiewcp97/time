package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Backup original env vars
	origDB := os.Getenv("DATABASE_URL")
	origRedis := os.Getenv("REDIS_URL")
	origPort := os.Getenv("PORT")
	origGemini := os.Getenv("GEMINI_API_KEY")
	origModel := os.Getenv("LLM_MODEL")
	origRole := os.Getenv("ROLE")

	defer func() {
		// Restore env
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("REDIS_URL", origRedis)
		os.Setenv("PORT", origPort)
		os.Setenv("GEMINI_API_KEY", origGemini)
		os.Setenv("LLM_MODEL", origModel)
		os.Setenv("ROLE", origRole)
	}()

	// Clear environment for test
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("REDIS_URL")
	os.Unsetenv("PORT")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("LLM_MODEL")
	os.Unsetenv("ROLE")

	// Backup any existing .env or config.json in working directory
	if _, err := os.Stat(".env"); err == nil {
		os.Rename(".env", ".env.bak")
		defer os.Rename(".env.bak", ".env")
	}
	if _, err := os.Stat("config.json"); err == nil {
		os.Rename("config.json", "config.json.bak")
		defer os.Rename("config.json.bak", "config.json")
	}

	// 1. Test Defaults
	Load()

	if AppConfig.Port != "8080" {
		t.Errorf("Expected default Port to be 8080, got %s", AppConfig.Port)
	}
	if AppConfig.LLMModel != "gemini-1.5-flash" {
		t.Errorf("Expected default LLMModel to be gemini-1.5-flash, got %s", AppConfig.LLMModel)
	}
	if AppConfig.Role != "both" {
		t.Errorf("Expected default Role to be both, got %s", AppConfig.Role)
	}

	// 2. Test Env File loading
	envContent := "PORT=9090\nGEMINI_API_KEY=test_key\n"
	err := os.WriteFile(".env", []byte(envContent), 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(".env")

	// Reset AppConfig & clear env so we reload
	AppConfig = Config{}
	os.Unsetenv("PORT")
	os.Unsetenv("GEMINI_API_KEY")

	Load()
	if AppConfig.Port != "9090" {
		t.Errorf("Expected Port to be 9090 from .env, got %s", AppConfig.Port)
	}
	if AppConfig.GeminiAPIKey != "test_key" {
		t.Errorf("Expected GeminiAPIKey to be test_key from .env, got %s", AppConfig.GeminiAPIKey)
	}

	// 3. Test config.json loading
	jsConfig := Config{
		Port:     "7070",
		LLMModel: "gemini-1.5-pro",
	}
	jsData, err := json.Marshal(jsConfig)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile("config.json", jsData, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("config.json")

	AppConfig = Config{}
	os.Unsetenv("PORT")
	os.Unsetenv("LLM_MODEL")

	Load()

	// 4. Test OS env override (highest precedence)
	os.Setenv("PORT", "6060")
	Load()
	if AppConfig.Port != "6060" {
		t.Errorf("Expected Port to be 6060 overridden by OS env, got %s", AppConfig.Port)
	}
}
