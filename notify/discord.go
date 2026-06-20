package notify

import (
	"context"
	"fmt"
	"sync"

	"github.com/ardean/mdadm-notifier/config"
	"github.com/bwmarrin/discordgo"
)

type Discord struct {
	channelID string
	session   *discordgo.Session
	ready     chan struct{}
	once      sync.Once
}

func NewDiscord(cfg config.DiscordConfig) (*Discord, error) {
	session, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("create discord client: %w", err)
	}

	d := &Discord{
		channelID: cfg.ChannelID,
		session:   session,
		ready:     make(chan struct{}),
	}

	session.Identify.Intents = discordgo.IntentsDirectMessages
	session.AddHandler(func(_ *discordgo.Session, _ *discordgo.Ready) {
		d.once.Do(func() { close(d.ready) })
	})

	return d, nil
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) Start() error {
	if err := d.session.Open(); err != nil {
		return fmt.Errorf("open discord connection: %w", err)
	}

	<-d.ready
	return nil
}

func (d *Discord) Send(_ context.Context, message string) error {
	for _, part := range SplitMessage(message, DiscordMessageLimit) {
		if _, err := d.session.ChannelMessageSend(d.channelID, part); err != nil {
			return fmt.Errorf("send discord message: %w", err)
		}
	}
	return nil
}

func (d *Discord) Close() error {
	return d.session.Close()
}
