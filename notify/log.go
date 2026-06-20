package notify

import (
	"context"
	"log"
)

type Log struct{}

func (Log) Name() string { return "log" }

func (Log) Send(_ context.Context, message string) error {
	log.Printf("notify-log: %s", message)
	return nil
}

func (Log) Close() error { return nil }
