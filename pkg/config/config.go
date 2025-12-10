package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	Logger    LoggerConfig    `mapstructure:"logger"`
}

type ServerConfig struct {
	Port            int    `mapstructure:"port"`
	Host            string `mapstructure:"host"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"ssl_mode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	ConsumerGroup string   `mapstructure:"consumer_group"`
	Topic         string   `mapstructure:"topic"`
}

type AuthConfig struct {
	JWTSecret        string `mapstructure:"jwt_secret"`
	JWTExpiry        int    `mapstructure:"jwt_expiry"`
	RefreshExpiry    int    `mapstructure:"refresh_expiry"`
	PrivateKeyPath   string `mapstructure:"private_key_path"`
	PublicKeyPath    string `mapstructure:"public_key_path"`
	JWT              JWTConfig `mapstructure:"jwt"`
}

type JWTConfig struct {
	SecretKey    string `mapstructure:"secret_key"`
	ExpiryHours  int    `mapstructure:"expiry_hours"`
	RefreshDays  int    `mapstructure:"refresh_days"`
	Issuer       string `mapstructure:"issuer"`
	Algorithm    string `mapstructure:"algorithm"` // HS256 for dev, RS256 for prod
}

type TelemetryConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	JaegerURL    string `mapstructure:"jaeger_url"`
	ServiceName  string `mapstructure:"service_name"`
	SamplingRate float64 `mapstructure:"sampling_rate"`
}

type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	AddCaller  bool   `mapstructure:"add_caller"`
	Stacktrace bool   `mapstructure:"stacktrace"`
}

func Load(serviceName string) (*Config, error) {
	viper.SetConfigName(serviceName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/linkflow")
	
	// Set defaults
	setDefaults()
	
	// Enable environment variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetEnvPrefix("LINKFLOW")
	
	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we'll use defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Override with environment variables
	overrideFromEnv(&config)
	
	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.shutdown_timeout", 30)
	
	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "linkflow")
	viper.SetDefault("database.password", "linkflow123")
	viper.SetDefault("database.name", "linkflow")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 25)
	
	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)
	
	// Kafka defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.consumer_group", "linkflow-group")
	
	// Auth defaults
	viper.SetDefault("auth.jwt_expiry", 900)        // 15 minutes
	viper.SetDefault("auth.refresh_expiry", 604800) // 7 days
	viper.SetDefault("auth.jwt.secret_key", "development-secret-key-change-in-production")
	viper.SetDefault("auth.jwt.expiry_hours", 1)   // 1 hour for access token
	viper.SetDefault("auth.jwt.refresh_days", 7)    // 7 days for refresh token
	viper.SetDefault("auth.jwt.issuer", "linkflow-auth")
	viper.SetDefault("auth.jwt.algorithm", "HS256") // HS256 for dev, RS256 for prod
	
	// Telemetry defaults
	viper.SetDefault("telemetry.enabled", true)
	viper.SetDefault("telemetry.jaeger_url", "http://localhost:14268/api/traces")
	viper.SetDefault("telemetry.sampling_rate", 1.0)
	
	// Logger defaults
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.format", "json")
	viper.SetDefault("logger.output", "stdout")
	viper.SetDefault("logger.add_caller", true)
	viper.SetDefault("logger.stacktrace", false)
}

func overrideFromEnv(cfg *Config) {
	// Override specific fields from environment variables
	// Viper automatically reads LINKFLOW_DATABASE_HOST, LINKFLOW_DATABASE_PORT, etc
	if host := viper.GetString("DATABASE_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := viper.GetInt("DATABASE_PORT"); port != 0 {
		cfg.Database.Port = port
	}
	if user := viper.GetString("DATABASE_USER"); user != "" {
		cfg.Database.User = user
	}
	if pass := viper.GetString("DATABASE_PASSWORD"); pass != "" {
		cfg.Database.Password = pass
	}
	if name := viper.GetString("DATABASE_NAME"); name != "" {
		cfg.Database.Name = name
	}
	
	if redisHost := viper.GetString("REDIS_HOST"); redisHost != "" {
		cfg.Redis.Host = redisHost
	}
	if redisPort := viper.GetInt("REDIS_PORT"); redisPort != 0 {
		cfg.Redis.Port = redisPort
	}
	
	if brokers := viper.GetString("KAFKA_BROKERS"); brokers != "" {
		cfg.Kafka.Brokers = strings.Split(brokers, ",")
	}
	
	if servicePort := viper.GetInt("SERVER_PORT"); servicePort != 0 {
		cfg.Server.Port = servicePort
	}
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
