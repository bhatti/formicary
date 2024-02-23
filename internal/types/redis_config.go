package types

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"time"
)

// RedisConfig redis config
type RedisConfig struct {
	Host       string        `yaml:"host" mapstructure:"host" env:"HOST"`
	Port       int           `yaml:"port" mapstructure:"port" env:"PORT"`
	Password   string        `yaml:"password" mapstructure:"password" env:"PASSWORD"`
	TTLMinutes time.Duration `yaml:"ttl_minutes" mapstructure:"ttl_minutes"`
	PoolSize   int           `yaml:"pool_size" mapstructure:"pool_size"`
	MaxPopWait time.Duration `yaml:"max_subscription_wait" mapstructure:"max_subscription_wait"`
}

// GetPool returns redis pool
func (c *RedisConfig) GetPool() (pool *redis.Pool, err error) {
	if c.Host == "" || c.Port == 0 {
		return nil, fmt.Errorf("redis is not configured %s:%d", c.Host, c.Port)
	}
	hostPort := fmt.Sprintf("%s:%d", c.Host, c.Port)
	pool = &redis.Pool{
		MaxIdle:   c.PoolSize,
		MaxActive: c.PoolSize,
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", hostPort)
			if err != nil {
				return nil, err
			}
			if c.Password != "" {
				if _, err := conn.Do("AUTH", c.Password); err != nil {
					_ = conn.Close()
					return nil, err
				}
			}
			return conn, err
		},
		TestOnBorrow: func(conn redis.Conn, t time.Time) error {
			_, err := conn.Do("PING")
			return err
		},
	}
	return
}

// Validate - validates
func (c *RedisConfig) Validate() error {
	if c.Host == "" {
		return errors.New("redis host is not set")
	}
	if c.Port == 0 {
		c.Port = 6379
	}
	if c.MaxPopWait == 0 {
		c.MaxPopWait = 60 * time.Second
	}
	return nil
}
