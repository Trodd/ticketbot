package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the bot
type Config struct {
	Token          string
	GuildID        string
	TicketCategory string
	SupportRoleID  string
	LogChannelID   string
}

// Load reads configuration from environment variables
func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		Token:          getEnv("DISCORD_TOKEN", ""),
		GuildID:        getEnv("GUILD_ID", ""),
		TicketCategory: getEnv("TICKET_CATEGORY_ID", ""),
		SupportRoleID:  getEnv("SUPPORT_ROLE_ID", ""),
		LogChannelID:   getEnv("LOG_CHANNEL_ID", ""),
	}

	if cfg.Token == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
