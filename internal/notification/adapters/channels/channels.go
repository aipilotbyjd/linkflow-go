package channels

import (
	"context"
	"fmt"
)

type EmailChannel struct {
	smtpHost string
	smtpPort int
}

func NewEmailChannel(config interface{}) *EmailChannel {
	return &EmailChannel{}
}

func (c *EmailChannel) Send(ctx context.Context, recipient string, message interface{}) error {
	fmt.Printf("Sending email to %s\n", recipient)
	return nil
}

type SMSChannel struct {
	twilioSID   string
	twilioToken string
}

func NewSMSChannel(config interface{}) *SMSChannel {
	return &SMSChannel{}
}

func (c *SMSChannel) Send(ctx context.Context, recipient string, message interface{}) error {
	fmt.Printf("Sending SMS to %s\n", recipient)
	return nil
}

type SlackChannel struct {
	token string
}

func NewSlackChannel(token string) *SlackChannel {
	return &SlackChannel{token: token}
}

func (c *SlackChannel) Send(ctx context.Context, recipient string, message interface{}) error {
	fmt.Printf("Sending Slack message to %s\n", recipient)
	return nil
}

type PushChannel struct {
	fcmKey string
}

func NewPushChannel(config interface{}) *PushChannel {
	return &PushChannel{}
}

func (c *PushChannel) Send(ctx context.Context, recipient string, message interface{}) error {
	fmt.Printf("Sending push notification to %s\n", recipient)
	return nil
}

type TeamsChannel struct {
	webhookURL string
}

func NewTeamsChannel() *TeamsChannel {
	return &TeamsChannel{}
}

func (c *TeamsChannel) Send(ctx context.Context, recipient string, message interface{}) error {
	fmt.Printf("Sending Teams message to %s\n", recipient)
	return nil
}

type DiscordChannel struct {
	webhookURL string
}

func NewDiscordChannel() *DiscordChannel {
	return &DiscordChannel{}
}

func (c *DiscordChannel) Send(ctx context.Context, recipient string, message interface{}) error {
	fmt.Printf("Sending Discord message to %s\n", recipient)
	return nil
}
