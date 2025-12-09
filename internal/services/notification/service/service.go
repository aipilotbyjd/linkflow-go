package service

import (
	"context"

	"github.com/linkflow-go/internal/services/notification/repository"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type Channel interface {
	Send(ctx context.Context, recipient string, message interface{}) error
}

type NotificationService struct {
	repo          *repository.NotificationRepository
	eventBus      events.EventBus
	redis         *redis.Client
	logger        logger.Logger
	emailChannel  Channel
	smsChannel    Channel
	slackChannel  Channel
	pushChannel   Channel
	teamsChannel  Channel
	discordChannel Channel
}

func NewNotificationService(
	repo *repository.NotificationRepository,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
	emailChannel, smsChannel, slackChannel, pushChannel, teamsChannel, discordChannel Channel,
) *NotificationService {
	return &NotificationService{
		repo:           repo,
		eventBus:       eventBus,
		redis:          redis,
		logger:         logger,
		emailChannel:   emailChannel,
		smsChannel:     smsChannel,
		slackChannel:   slackChannel,
		pushChannel:    pushChannel,
		teamsChannel:   teamsChannel,
		discordChannel: discordChannel,
	}
}

func (s *NotificationService) HandleEvent(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling notification event", "type", event.Type, "id", event.ID)
	// Process event and send notifications
	return nil
}

func (s *NotificationService) SendNotification(ctx context.Context, channel string, recipient string, message interface{}) error {
	switch channel {
	case "email":
		return s.emailChannel.Send(ctx, recipient, message)
	case "sms":
		return s.smsChannel.Send(ctx, recipient, message)
	case "slack":
		return s.slackChannel.Send(ctx, recipient, message)
	case "push":
		return s.pushChannel.Send(ctx, recipient, message)
	case "teams":
		return s.teamsChannel.Send(ctx, recipient, message)
	case "discord":
		return s.discordChannel.Send(ctx, recipient, message)
	default:
		return nil
	}
}
