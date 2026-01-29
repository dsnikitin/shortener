package audit

type Config struct {
	FilePath         string `env:"FILE"`
	URL              string `env:"URL"`
	EventsLimit      int    `env:"EVENTS_LIMIT"`
	FileConsumerID   string `env:"FILE_CONSUMER_ID"`
	RemoteConsumerID string `env:"REMOTE_CONSUMER_ID"`
}
