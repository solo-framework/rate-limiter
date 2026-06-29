package config

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	Port            uint          // TCP port
	HttpPort        uint          // HTTP server port
	HttpMaxBodySize int64         // max body size for control methods (bytes)
	MaxConnections  int           // maximum number of connections
	WorkerPoolSize  int           // number of workers
	ReadTimeout     time.Duration // read timeout from client connection
	WriteTimeout    time.Duration // write timeout to client connection
	ShutdownTimeout time.Duration // max wait time for server graceful shutdown
	PacketSize      int           // max input message size in bytes
	CleanupInterval time.Duration // limiter TTL cleanup check interval
	TTL             time.Duration // TokenLimiter lifetime (shared for all groups)
	HttpSecret      string        `log:"false"` // secret key for HTTP control methods
	BasicAuthUser   string        `log:"false"` // basic auth username
	BasicAuthPass   string        `log:"false"` // basic auth password
	StorePath       string        // path to store limiters
	Groups          GroupList     // list of groups

}

var globalConfig *Config

func AppConfig() *Config {
	return globalConfig
}

func (s *Config) String() string {

	sb := strings.Builder{}

	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := val.Type().Field(i)

		if !fieldType.IsExported() {
			continue
		}

		if tag := fieldType.Tag.Get("log"); tag == "false" {
			// field.SetString("[censored]")
			fmt.Fprintf(&sb, "%s: [censored], ", fieldType.Name)
			continue
		}

		if field.CanInterface() {
			fmt.Fprintf(&sb, "%s: %+v, ", fieldType.Name, field.Interface())
		}
	}
	return strings.TrimRight(sb.String(), ", ")
}

// configDTO describes a flat structure for proper config file decoding.
type configDTO struct {
	Port            uint          `mapstructure:"port"               validate:"required,port"`
	HttpPort        uint          `mapstructure:"http_port"          validate:"required,port"`
	HttpMaxBodySize int64         `mapstructure:"http_max_body_size" validate:"required,min=1"`
	MaxConnections  int           `mapstructure:"max_connections"    validate:"required,min=1,max=1000000"`
	WorkerPoolSize  int           `mapstructure:"worker_pool_size"   validate:"required,min=1"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"       validate:"required,gt=0"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"      validate:"required,gt=0"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"   validate:"required,gt=0"`
	PacketSize      int           `mapstructure:"packet_size"        validate:"required,gt=0"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"   validate:"required,gt=0"`
	TTL             time.Duration `mapstructure:"ttl"                validate:"required,gt=0"`
	HttpSecret      string        `mapstructure:"http_secret"        validate:"required,min=6,max=256"`
	BasicAuthUser   string        `mapstructure:"basic_auth_user"    validate:"required,min=1,max=256"`
	BasicAuthPass   string        `mapstructure:"basic_auth_pass"    validate:"required,min=6,max=256"`
	StorePath       string        `mapstructure:"store_path"         validate:"required,min=1,max=1024"`
	Groups          []groupsDTO   `validate:"required,dive,required"`
}

type groupsDTO struct {
	Name  string  `validate:"required,min=1,max=64"`
	Rate  float64 `validate:"required,gt=0"`
	Burst int     `validate:"required,gt=0"`
}

func LoadConfig(path string) error {

	// see https://deepwiki.com/spf13/viper/1.2-key-concepts#precedence-rules
	v := viper.New()
	v.SetConfigFile(path)

	// ENV setup: RATE_LIMITER prefix for all env variables (for example RATE_LIMITER_PORT)
	v.SetEnvPrefix("RATE_LIMITER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("port", 49105)
	v.SetDefault("http_port", 8090)
	v.SetDefault("http_max_body_size", 1024)
	v.SetDefault("max_connections", 1000)
	v.SetDefault("worker_pool_size", 100)
	v.SetDefault("read_timeout", "100ms")
	v.SetDefault("write_timeout", "100ms")
	v.SetDefault("shutdown_timeout", "5s")
	v.SetDefault("packet_size", 512)
	v.SetDefault("cleanup_interval", "12h")
	v.SetDefault("ttl", "48h")
	v.SetDefault("http_secret", "secret")
	v.SetDefault("basic_auth_user", "user")
	v.SetDefault("basic_auth_pass", "password")
	v.SetDefault("store_path", "./")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config error: %w", err)
	}

	var dto configDTO
	if err := v.Unmarshal(&dto); err != nil {
		return fmt.Errorf("unmarshal config error: %w", err)
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(&dto); err != nil {
		res := strings.Builder{}

		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			for _, e := range ve {
				fmt.Fprintf(&res, "%s has wrong value `%v` [must be %s %s] ", e.Field(), e.Value(), e.Tag(), e.Param())
				// return fmt.Errorf("%s has wrong value `%v` [must be %s %s] ", e.Field(), e.Value(), e.Tag(), e.Param())
			}
		}
		return fmt.Errorf("config validation error: %s", &res)
	}

	// Build final config structure
	cfg := &Config{
		Port:            dto.Port,
		HttpPort:        dto.HttpPort,
		HttpMaxBodySize: dto.HttpMaxBodySize,
		MaxConnections:  dto.MaxConnections,
		WorkerPoolSize:  dto.WorkerPoolSize,
		ReadTimeout:     dto.ReadTimeout,
		WriteTimeout:    dto.WriteTimeout,
		ShutdownTimeout: dto.ShutdownTimeout,
		PacketSize:      dto.PacketSize,
		CleanupInterval: dto.CleanupInterval,
		TTL:             dto.TTL,
		Groups:          *NewGroupList(), //
		HttpSecret:      dto.HttpSecret,
		BasicAuthUser:   dto.BasicAuthUser,
		BasicAuthPass:   dto.BasicAuthPass,
		StorePath:       dto.StorePath,
	}

	// Fills groups via Add function to trigger group validation
	for _, g := range dto.Groups {
		err := cfg.Groups.Add(g.Name, g.Rate, g.Burst) //
		if err != nil {
			return fmt.Errorf("group validation error: %w", err)
		}
	}

	globalConfig = cfg
	return nil
}
