package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
}

type Config struct {
	Host         string
	Port         int
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

func New(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		QueryFields: true,
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (db *DB) Migrate(models ...interface{}) error {
	return db.AutoMigrate(models...)
}

func (db *DB) Transaction(fn func(*gorm.DB) error) error {
	return db.DB.Transaction(fn)
}

func (db *DB) WithContext(ctx context.Context) *gorm.DB {
	return db.DB.WithContext(ctx)
}

// Helper functions for common queries
func (db *DB) FindByID(ctx context.Context, dest interface{}, id string) error {
	return db.WithContext(ctx).Where("id = ?", id).First(dest).Error
}

func (db *DB) FindAll(ctx context.Context, dest interface{}, conditions ...interface{}) error {
	query := db.WithContext(ctx)
	if len(conditions) > 0 {
		query = query.Where(conditions[0], conditions[1:]...)
	}
	return query.Find(dest).Error
}

func (db *DB) Create(ctx context.Context, value interface{}) error {
	return db.WithContext(ctx).Create(value).Error
}

func (db *DB) Update(ctx context.Context, value interface{}) error {
	return db.WithContext(ctx).Save(value).Error
}

func (db *DB) Delete(ctx context.Context, value interface{}, conditions ...interface{}) error {
	query := db.WithContext(ctx)
	if len(conditions) > 0 {
		query = query.Where(conditions[0], conditions[1:]...)
	}
	return query.Delete(value).Error
}

func (db *DB) Count(ctx context.Context, model interface{}, conditions ...interface{}) (int64, error) {
	var count int64
	query := db.WithContext(ctx).Model(model)
	if len(conditions) > 0 {
		query = query.Where(conditions[0], conditions[1:]...)
	}
	err := query.Count(&count).Error
	return count, err
}

// Pagination helper
type Pagination struct {
	Limit int
	Page  int
	Sort  string
	Total int64
	Pages int
}

func (db *DB) Paginate(ctx context.Context, dest interface{}, pagination *Pagination, conditions ...interface{}) error {
	query := db.WithContext(ctx)

	// Apply conditions
	if len(conditions) > 0 {
		query = query.Where(conditions[0], conditions[1:]...)
	}

	// Count total records
	var total int64
	if err := query.Model(dest).Count(&total).Error; err != nil {
		return err
	}
	pagination.Total = total

	// Calculate pages
	if pagination.Limit > 0 {
		pagination.Pages = int((total + int64(pagination.Limit) - 1) / int64(pagination.Limit))
	}

	// Apply pagination
	if pagination.Limit > 0 {
		query = query.Limit(pagination.Limit)
	}
	if pagination.Page > 0 && pagination.Limit > 0 {
		offset := (pagination.Page - 1) * pagination.Limit
		query = query.Offset(offset)
	}

	// Apply sorting
	if pagination.Sort != "" {
		query = query.Order(pagination.Sort)
	}

	return query.Find(dest).Error
}
