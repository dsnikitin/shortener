package audit

// Config содержит конфигурационные параметры для аудита.
type Config struct {
	FilePath         string `env:"FILE" json:"file"`
	URL              string `env:"URL" json:"url"`
	EventsLimit      int    `env:"EVENTS_LIMIT" json:"events_limit"`
	FileConsumerID   string `env:"FILE_CONSUMER_ID" json:"file_consumer_id"`
	RemoteConsumerID string `env:"REMOTE_CONSUMER_ID" json:"remote_consumer_id"`
}
