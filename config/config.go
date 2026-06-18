package config

import (
	"log"
	"os"
)

type Config struct {
	Discord DiscordConfig
}

type DiscordConfig struct {
	Token     string
	ChannelID string
}

func Load() Config {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is missing")
	}

	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		log.Fatal("DISCORD_CHANNEL_ID is missing")
	}

	return Config{
		Discord: DiscordConfig{
			Token:     token,
			ChannelID: channelID,
		},
	}
}
