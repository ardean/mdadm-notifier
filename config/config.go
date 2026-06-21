package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	NotifyMethods         []string
	Discord               DiscordConfig
	Webhook               WebhookConfig
	MDDevice              string
	CheckInterval         time.Duration
	SelfTestEnabled       bool
	SelfTestCheckInterval time.Duration
	SelfTestShortInterval time.Duration
	SelfTestLongInterval  time.Duration
	SelfTestMinGap        time.Duration
	NotifyStartupShutdown bool
	NotifyReminderInterval time.Duration
	Hostname              string
	SmartStateDir         string
	SmartReallocatedThreshold   int
	SmartUncorrectableThreshold int
	SmartPendingThreshold       int
	SmartOfflineThreshold       int
	SmartErrorLogThreshold      int
	WebEnabled                  bool
	WebAddr                     string
}

type DiscordConfig struct {
	Token     string
	ChannelID string
}

type WebhookConfig struct {
	URL string
}

func Load() Config {
	methods := loadNotifyMethods()
	cfg := Config{
		NotifyMethods:         methods,
		MDDevice:              loadMDDevice(),
		CheckInterval:         loadDuration("CHECK_INTERVAL", time.Hour),
		SelfTestEnabled:       loadBool("SELFTEST_ENABLED", true),
		SelfTestCheckInterval: loadDuration("SELFTEST_CHECK_INTERVAL", time.Hour),
		SelfTestShortInterval: loadDuration("SELFTEST_SHORT_INTERVAL", 7*24*time.Hour),
		SelfTestLongInterval:  loadDuration("SELFTEST_LONG_INTERVAL", 30*24*time.Hour),
		SelfTestMinGap:        loadDuration("SELFTEST_MIN_GAP", 24*time.Hour),
		NotifyStartupShutdown: loadBool("NOTIFY_STARTUP_SHUTDOWN", true),
		NotifyReminderInterval: loadDuration("NOTIFY_REMINDER_INTERVAL", 0),
		Hostname:              loadHostname(),
		SmartStateDir:         loadSmartStateDir(),
		SmartReallocatedThreshold:   loadInt("SMART_REALLOCATED_THRESHOLD", 1),
		SmartUncorrectableThreshold: loadInt("SMART_UNCORRECTABLE_THRESHOLD", 1),
		SmartPendingThreshold:       loadInt("SMART_PENDING_THRESHOLD", 1),
		SmartOfflineThreshold:       loadInt("SMART_OFFLINE_THRESHOLD", 1),
		SmartErrorLogThreshold:      loadInt("SMART_ERROR_LOG_THRESHOLD", 1),
		WebEnabled:                  loadBool("WEB_ENABLED", true),
		WebAddr:                     loadWebAddr(),
	}

	if containsMethod(methods, "discord") {
		cfg.Discord = loadDiscordConfig()
	}
	if containsMethod(methods, "webhook") {
		cfg.Webhook = loadWebhookConfig()
	}

	return cfg
}

func loadNotifyMethods() []string {
	raw := os.Getenv("NOTIFY_METHODS")
	if raw == "" {
		return []string{"discord"}
	}

	parts := strings.Split(raw, ",")
	methods := make([]string, 0, len(parts))
	for _, part := range parts {
		method := strings.ToLower(strings.TrimSpace(part))
		if method == "" {
			continue
		}
		methods = append(methods, method)
	}

	if len(methods) == 0 {
		log.Fatal("NOTIFY_METHODS is empty")
	}

	return methods
}

func loadDiscordConfig() DiscordConfig {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is missing")
	}

	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		log.Fatal("DISCORD_CHANNEL_ID is missing")
	}

	return DiscordConfig{
		Token:     token,
		ChannelID: channelID,
	}
}

func loadWebhookConfig() WebhookConfig {
	url := os.Getenv("WEBHOOK_URL")
	if url == "" {
		log.Fatal("WEBHOOK_URL is missing")
	}

	return WebhookConfig{URL: url}
}

func loadMDDevice() string {
	mdDevice := os.Getenv("MD_DEVICE")
	if mdDevice == "" {
		return "/dev/md0"
	}
	return mdDevice
}

func containsMethod(methods []string, method string) bool {
	for _, item := range methods {
		if item == method {
			return true
		}
	}
	return false
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

func loadBool(name string, defaultVal bool) bool {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultVal
	}
	return strings.EqualFold(raw, "true") || raw == "1"
}

func loadInt(name string, defaultVal int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return defaultVal
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		log.Fatalf("%s is invalid: %v", name, err)
	}

	return parsed
}

func loadSmartStateDir() string {
	raw := os.Getenv("SMART_STATE_DIR")
	if raw == "" {
		return "/data"
	}
	return raw
}

func loadWebAddr() string {
	if addr := os.Getenv("WEB_ADDR"); addr != "" {
		return addr
	}

	port := os.Getenv("WEB_PORT")
	if port == "" {
		port = "8080"
	}
	return ":" + port
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
