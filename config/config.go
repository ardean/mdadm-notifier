package config

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	Discord       DiscordConfig
	MDDevice      string
	CheckInterval time.Duration
	Hostname      string
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

	mdDevice := os.Getenv("MD_DEVICE")
	if mdDevice == "" {
		mdDevice = "/dev/md0"
	}

	checkInterval := time.Hour
	if raw := os.Getenv("CHECK_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatalf("CHECK_INTERVAL is invalid: %v", err)
		}
		checkInterval = parsed
	}


	return Config{
		Discord: DiscordConfig{
			Token:     token,
			ChannelID: channelID,
		},
		MDDevice:      mdDevice,
		CheckInterval: checkInterval,
		Hostname:      loadHostname(),
	}
}

func loadHostname() string {
	if hostname := os.Getenv("SERVER_HOSTNAME"); hostname != "" {
		return hostname
	}

	if data, err := os.ReadFile("/etc/hostname"); err == nil {
		if hostname := strings.TrimSpace(string(data)); hostname != "" {
			return hostname
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("could not determine hostname: %v", err)
		return "unknown"
	}

	return hostname
}
