package openai

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	APIKey  string        `envconfig:"OPENAI_API_KEY" required:"true"`
	Timeout time.Duration `envconfig:"OPENAI_TIMEOUT"                 default:"30s"`
}

func (c *Client) parseConfig() error {
	if err := envconfig.Process("", &c.cfg); err != nil {
		return fmt.Errorf("envconfig.Process error: %w", err)
	}

	return nil
}
