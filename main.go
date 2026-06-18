package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ardean/mdadm-notifier/config"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	session.Identify.Intents = discordgo.IntentsDirectMessages

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Printf("%s is connected!\n", r.User.Username)

		raidInfo := getRaidInfo()
		sendMessage(s, cfg, raidInfo)
	})

	if err := session.Open(); err != nil {
		log.Fatalf("Error opening connection: %v", err)
	}
	defer session.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
}

func sendMessage(s *discordgo.Session, cfg config.Config, message string) {
	fmt.Printf("sending: %s\n", message)

	if _, err := s.ChannelMessageSend(cfg.Discord.ChannelID, message); err != nil {
		fmt.Printf("Error sending message: %v\n", err)
	}
}

func getRaidInfo() string {
	return runCommand("mdadm", "-D", "/dev/md0")
}

func runCommand(command string, args ...string) string {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(exitErr.Stderr)
		}
		log.Fatalf("Failed to execute command: %v", err)
	}

	return string(output)
}
