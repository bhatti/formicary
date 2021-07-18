package events

import "time"

// BaseEvent common event properties
type BaseEvent struct {
	ID        string    `yaml:"-" json:"id" mapstructure:"id" gorm:"primary_key"`
	Source    string    `json:"source" mapstructure:"source"`
	EventType string    `json:"event_type" mapstructure:"event_type"`
	CreatedAt time.Time `json:"created_at" mapstructure:"created_at"`
}
