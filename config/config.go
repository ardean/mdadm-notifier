package config

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	Discord               DiscordConfig
	MDDevice              string
	CheckInterval         time.Duration
	SelfTestEnabled       bool
	SelfTestCheckInterval time.Duration
	SelfTestShortInterval time.Duration
	SelfTestLongInterval  time.Duration
	NotifyStartupShutdown bool
	Hostname              string
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

	selfTestEnabled := true
	if raw := os.Getenv("SELFTEST_ENABLED"); raw != "" {
		selfTestEnabled = strings.EqualFold(raw, "true") || raw == "1"
	}

	notifyStartupShutdown := true
	if raw := os.Getenv("NOTIFY_STARTUP_SHUTDOWN"); raw != "" {
		notifyStartupShutdown = strings.EqualFold(raw, "true") || raw == "1"
	}

	return Config{
		Discord: DiscordConfig{
			Token:     token,
			ChannelID: channelID,
		},
		MDDevice:              mdDevice,
		CheckInterval:         checkInterval,
		SelfTestEnabled:       selfTestEnabled,
		SelfTestCheckInterval: loadDuration("SELFTEST_CHECK_INTERVAL", time.Hour),
		SelfTestShortInterval: loadDuration("SELFTEST_SHORT_INTERVAL", 7*24*time.Hour),
		SelfTestLongInterval:  loadDuration("SELFTEST_LONG_INTERVAL", 30*24*time.Hour),
		NotifyStartupShutdown: notifyStartupShutdown,
		Hostname:              loadHostname(),
	}
}

func loadDuration(name string, defaultVal time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultVal
	}
	if raw == "0" {
		return 0
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("%s is invalid: %v", name, err)
	}

	return parsed
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
