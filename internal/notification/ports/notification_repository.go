package ports

import "context"

type NotificationRepository interface {
	CreateNotification(ctx context.Context, notification interface{}) error
	GetNotifications(ctx context.Context, userID string) ([]interface{}, error)
	MarkAsRead(ctx context.Context, id string) error
}
