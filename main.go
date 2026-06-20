package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ardean/mdadm-notifier/config"
	"github.com/ardean/mdadm-notifier/format"
	"github.com/ardean/mdadm-notifier/mdadm"
	"github.com/ardean/mdadm-notifier/smart"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	if reason := run(); reason != "" {
		log.Printf("exiting: %s", reason)
		os.Exit(1)
	}
}

func run() string {
	_ = godotenv.Load()
	cfg := config.Load()

	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	var (
		started    bool
		exitReason string
	)

	defer func() {
		if r := recover(); r != nil {
			exitReason = fmt.Sprintf("unexpected error: %v", r)
		}
		if started {
			notifyLifecycle(session, cfg, formatShutdownMessage(exitReason))
		}
		session.Close()
	}()

	session.Identify.Intents = discordgo.IntentsDirectMessages

	var startupOnce sync.Once
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		startupOnce.Do(func() {
			started = true
			fmt.Printf("%s is connected!\n", r.User.Username)
			notifyLifecycle(s, cfg, formatStartupMessage(cfg))
			runHealthCheck(s, cfg)
			if cfg.SelfTestEnabled {
				runSelfTestCycle(s, cfg)
			}
		})
	})

	if err := session.Open(); err != nil {
		return fmt.Sprintf("failed to open Discord connection: %v", err)
	}

	go runPeriodicHealthChecks(session, cfg)
	if cfg.SelfTestEnabled {
		go runPeriodicSelfTests(session, cfg)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	exitReason = fmt.Sprintf("shutdown (%s)", sig)
	return ""
}

func formatStartupMessage(cfg config.Config) string {
	msg := fmt.Sprintf("Watcher started — monitoring %s every %s", cfg.MDDevice, format.Duration(cfg.CheckInterval))
	if cfg.SelfTestEnabled {
		msg += fmt.Sprintf("; self-tests every %s (short %s, long %s, min gap %s)",
			format.Duration(cfg.SelfTestCheckInterval),
			format.Duration(cfg.SelfTestShortInterval),
			format.Duration(cfg.SelfTestLongInterval),
			format.Duration(cfg.SelfTestMinGap))
	}
	return msg
}

func formatShutdownMessage(reason string) string {
	if reason == "" {
		return "Watcher stopped — normal shutdown"
	}
	return fmt.Sprintf("Watcher stopped — %s", reason)
}

func runPeriodicHealthChecks(session *discordgo.Session, cfg config.Config) {
	ticker := time.NewTicker(cfg.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		runHealthCheck(session, cfg)
	}
}

func runHealthCheck(session *discordgo.Session, cfg config.Config) {
	log.Printf("health check: checking %s", cfg.MDDevice)

	raidHealth, err := mdadm.CheckHealth(cfg.MDDevice)
	if err != nil {
		log.Printf("health check: raid check failed: %v", err)
		sendMessage(session, cfg, fmt.Sprintf("RAID health check failed for %s: %v", cfg.MDDevice, err))
		return
	}

	if raidHealth.Healthy {
		log.Printf("health check: raid array healthy")
	} else {
		log.Printf("health check: raid array unhealthy: %s", strings.Join(raidHealth.Issues, ", "))
	}

	var unhealthyDisks []smart.Result
	for _, device := range raidHealth.Devices {
		result := smart.CheckDevice(device)
		if cfg.SelfTestEnabled {
			result = smart.EnrichWithSelfTest(result)
		}
		if result.Healthy {
			log.Printf("health check: %s", result.Summary)
			continue
		}
		log.Printf("health check: unhealthy disk: %s", result.Summary)
		unhealthyDisks = append(unhealthyDisks, result)
	}

	if raidHealth.Healthy && len(unhealthyDisks) == 0 {
		return
	}

	var message strings.Builder
	message.WriteString("Health check found issues:\n")

	if !raidHealth.Healthy {
		message.WriteString("\nRAID issues:\n")
		message.WriteString(strings.Join(raidHealth.Issues, "\n"))
		message.WriteString("\n\n")
		message.WriteString(raidHealth.Detail)
	}

	for _, result := range unhealthyDisks {
		message.WriteString("\n\n")
		message.WriteString(result.Summary)
	}

	sendMessage(session, cfg, message.String())
}

func runPeriodicSelfTests(session *discordgo.Session, cfg config.Config) {
	ticker := time.NewTicker(cfg.SelfTestCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		runSelfTestCycle(session, cfg)
	}
}

func runSelfTestCycle(session *discordgo.Session, cfg config.Config) {
	log.Printf("self-test: checking member disks for %s", cfg.MDDevice)

	raidHealth, err := mdadm.CheckHealth(cfg.MDDevice)
	if err != nil {
		log.Printf("self-test: raid check failed: %v", err)
		sendMessage(session, cfg, fmt.Sprintf("Self-test scheduling failed for %s: %v", cfg.MDDevice, err))
		return
	}

	for _, device := range raidHealth.Devices {
		device = smart.NormalizeDevice(device)

		testLog, err := smart.ReadSelfTestLog(device)
		if err != nil {
			log.Printf("self-test: %s: %v", device, err)
			sendMessage(session, cfg, fmt.Sprintf("Self-test log read failed for %s: %v", device, err))
			continue
		}

		deviceInfo, err := smart.ReadDeviceInfo(device)
		if err != nil {
			log.Printf("self-test: %s: %v", device, err)
		} else {
			testLog.PowerOnHours = deviceInfo.PowerOnHours
			if deviceInfo.InProgress {
				testLog.InProgress = true
			}
		}

		if testLog.InProgress {
			log.Printf("self-test: %s: test already in progress", device)
			continue
		}

		if cfg.SelfTestLongInterval > 0 && testLog.LongDue(cfg.SelfTestLongInterval, cfg.SelfTestMinGap) {
			if err := smart.StartSelfTest(device, "long"); err != nil {
				if smart.IsSelfTestInProgress(err) {
					log.Printf("self-test: %s: test already in progress", device)
					continue
				}
				log.Printf("self-test: %s: failed to start long test: %v", device, err)
				sendMessage(session, cfg, fmt.Sprintf("Failed to start long self-test on %s: %v", device, err))
				continue
			}
			log.Printf("self-test: %s: started long test", device)
			continue
		}

		if cfg.SelfTestShortInterval > 0 && testLog.ShortDue(cfg.SelfTestShortInterval, cfg.SelfTestMinGap) {
			if err := smart.StartSelfTest(device, "short"); err != nil {
				if smart.IsSelfTestInProgress(err) {
					log.Printf("self-test: %s: test already in progress", device)
					continue
				}
				log.Printf("self-test: %s: failed to start short test: %v", device, err)
				sendMessage(session, cfg, fmt.Sprintf("Failed to start short self-test on %s: %v", device, err))
				continue
			}
			log.Printf("self-test: %s: started short test", device)
		}
	}
}

func notifyLifecycle(session *discordgo.Session, cfg config.Config, message string) {
	if !cfg.NotifyStartupShutdown {
		log.Printf("lifecycle: %s", message)
		return
	}

	sendMessage(session, cfg, message)
}

func sendMessage(s *discordgo.Session, cfg config.Config, message string) {
	full := fmt.Sprintf("[%s] %s", cfg.Hostname, message)
	fmt.Printf("sending: %s\n", full)

	if _, err := s.ChannelMessageSend(cfg.Discord.ChannelID, full); err != nil {
		fmt.Printf("Error sending message: %v\n", err)
	}
}
