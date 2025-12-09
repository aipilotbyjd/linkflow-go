package config

import (
	"github.com/linkflow-go/pkg/database"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// ToLoggerConfig converts LoggerConfig to logger.Config
func (c LoggerConfig) ToLoggerConfig() logger.Config {
	return logger.Config{
		Level:      c.Level,
		Format:     c.Format,
		Output:     c.Output,
		AddCaller:  c.AddCaller,
		Stacktrace: c.Stacktrace,
	}
}

// ToDatabaseConfig converts DatabaseConfig to database.Config
func (c DatabaseConfig) ToDatabaseConfig() database.Config {
	return database.Config{
		Host:         c.Host,
		Port:         c.Port,
		User:         c.User,
		Password:     c.Password,
		Name:         c.Name,
		SSLMode:      c.SSLMode,
		MaxOpenConns: c.MaxOpenConns,
		MaxIdleConns: c.MaxIdleConns,
	}
}

// ToKafkaConfig converts KafkaConfig to events.KafkaConfig
func (c KafkaConfig) ToKafkaConfig() events.KafkaConfig {
	return events.KafkaConfig{
		Brokers:       c.Brokers,
		Topic:         c.Topic,
		ConsumerGroup: c.ConsumerGroup,
	}
}
