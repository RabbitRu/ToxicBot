package gigachat

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type config struct {
	AuthKey string        `envconfig:"GIGACHAT_AUTH_KEY" required:"true"`
	Scope   string        `envconfig:"GIGACHAT_SCOPE"                    default:"GIGACHAT_API_PERS"`
	Model   string        `envconfig:"GIGACHAT_MODEL"                    default:"GigaChat-Pro"`
	Timeout time.Duration `envconfig:"GIGACHAT_TIMEOUT"                  default:"60s"`
}

func (c *Client) parseConfig() error {
	if err := envconfig.Process("", &c.cfg); err != nil {
		return fmt.Errorf("envconfig.Process error: %w", err)
	}

	return nil
}
