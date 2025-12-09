package repository

import (
	"context"
	
	"github.com/linkflow-go/pkg/database"
)

type NotificationRepository struct {
	db *database.DB
}

func NewNotificationRepository(db *database.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) CreateNotification(ctx context.Context, notification interface{}) error {
	return r.db.WithContext(ctx).Create(&notification).Error
}

func (r *NotificationRepository) GetNotifications(ctx context.Context, userID string) ([]interface{}, error) {
	var notifications []interface{}
	return notifications, nil
}

func (r *NotificationRepository) MarkAsRead(ctx context.Context, id string) error {
	return nil
}
