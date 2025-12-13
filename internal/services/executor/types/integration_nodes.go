package types

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"
)

// EmailNodeExecutor sends emails
type EmailNodeExecutor struct {
	BaseNodeExecutor
}

func NewEmailNodeExecutor() *EmailNodeExecutor {
	return &EmailNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 60 * time.Second},
	}
}

func (e *EmailNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	to, _ := params["to"].(string)
	subject, _ := params["subject"].(string)
	body, _ := params["body"].(string)
	smtpHost, _ := params["smtpHost"].(string)
	smtpPort, _ := params["smtpPort"].(string)
	username, _ := params["username"].(string)
	password, _ := params["password"].(string)
	from, _ := params["from"].(string)

	if smtpHost == "" {
		smtpHost = "localhost"
	}
	if smtpPort == "" {
		smtpPort = "587"
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, smtpHost)
	}

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, []byte(msg))
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	return map[string]interface{}{
		"success": true,
		"to":      to,
		"subject": subject,
	}, nil
}

// SlackNodeExecutor sends Slack messages
type SlackNodeExecutor struct {
	BaseNodeExecutor
	client *http.Client
}

func NewSlackNodeExecutor() *SlackNodeExecutor {
	return &SlackNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
		client:           &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *SlackNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	webhookURL, _ := params["webhookUrl"].(string)
	channel, _ := params["channel"].(string)
	text, _ := params["text"].(string)
	username, _ := params["username"].(string)
	iconEmoji, _ := params["iconEmoji"].(string)
	blocks, _ := params["blocks"].([]interface{})

	payload := map[string]interface{}{
		"text": text,
	}

	if channel != "" {
		payload["channel"] = channel
	}
	if username != "" {
		payload["username"] = username
	}
	if iconEmoji != "" {
		payload["icon_emoji"] = iconEmoji
	}
	if len(blocks) > 0 {
		payload["blocks"] = blocks
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send Slack message: %w", err)
	}
	defer resp.Body.Close()

	return map[string]interface{}{
		"success":    resp.StatusCode == 200,
		"statusCode": resp.StatusCode,
	}, nil
}

// DiscordNodeExecutor sends Discord messages
type DiscordNodeExecutor struct {
	BaseNodeExecutor
	client *http.Client
}

func NewDiscordNodeExecutor() *DiscordNodeExecutor {
	return &DiscordNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
		client:           &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *DiscordNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	webhookURL, _ := params["webhookUrl"].(string)
	content, _ := params["content"].(string)
	username, _ := params["username"].(string)
	avatarURL, _ := params["avatarUrl"].(string)
	embeds, _ := params["embeds"].([]interface{})

	payload := map[string]interface{}{
		"content": content,
	}

	if username != "" {
		payload["username"] = username
	}
	if avatarURL != "" {
		payload["avatar_url"] = avatarURL
	}
	if len(embeds) > 0 {
		payload["embeds"] = embeds
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send Discord message: %w", err)
	}
	defer resp.Body.Close()

	return map[string]interface{}{
		"success":    resp.StatusCode >= 200 && resp.StatusCode < 300,
		"statusCode": resp.StatusCode,
	}, nil
}

// TelegramNodeExecutor sends Telegram messages
type TelegramNodeExecutor struct {
	BaseNodeExecutor
	client *http.Client
}

func NewTelegramNodeExecutor() *TelegramNodeExecutor {
	return &TelegramNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
		client:           &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *TelegramNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	botToken, _ := params["botToken"].(string)
	chatID, _ := params["chatId"].(string)
	text, _ := params["text"].(string)
	parseMode, _ := params["parseMode"].(string)

	if parseMode == "" {
		parseMode = "HTML"
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": parseMode,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send Telegram message: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return map[string]interface{}{
		"success": resp.StatusCode == 200,
		"result":  result,
	}, nil
}

// TriggerNodeExecutor handles trigger nodes
type TriggerNodeExecutor struct {
	BaseNodeExecutor
	triggerType string
}

func NewTriggerNodeExecutor(triggerType string) *TriggerNodeExecutor {
	return &TriggerNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
		triggerType:      triggerType,
	}
}

func (e *TriggerNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	// Trigger nodes pass through input data
	return map[string]interface{}{
		"triggerType": e.triggerType,
		"data":        input,
		"timestamp":   time.Now().Format(time.RFC3339),
	}, nil
}

// RespondToWebhookExecutor responds to webhook requests
type RespondToWebhookExecutor struct {
	BaseNodeExecutor
}

func NewRespondToWebhookExecutor() *RespondToWebhookExecutor {
	return &RespondToWebhookExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *RespondToWebhookExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	statusCode, _ := params["statusCode"].(float64)
	body := params["body"]
	headers, _ := params["headers"].(map[string]interface{})

	if statusCode == 0 {
		statusCode = 200
	}

	return map[string]interface{}{
		"webhookResponse": map[string]interface{}{
			"statusCode": int(statusCode),
			"body":       body,
			"headers":    headers,
		},
	}, nil
}

// StopAndErrorExecutor stops execution with error
type StopAndErrorExecutor struct {
	BaseNodeExecutor
}

func NewStopAndErrorExecutor() *StopAndErrorExecutor {
	return &StopAndErrorExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *StopAndErrorExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	message, _ := params["message"].(string)
	errorType, _ := params["errorType"].(string)

	if message == "" {
		message = "Workflow stopped by StopAndError node"
	}
	if errorType == "" {
		errorType = "WORKFLOW_STOPPED"
	}

	return nil, fmt.Errorf("%s: %s", errorType, message)
}

// NoOpExecutor does nothing (pass-through)
type NoOpExecutor struct {
	BaseNodeExecutor
}

func NewNoOpExecutor() *NoOpExecutor {
	return &NoOpExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *NoOpExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	return input, nil
}
