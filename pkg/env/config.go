package env

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Spec hold configuration of the proxy to build
type Spec struct {
	Namespace string         `required:"true" envconfig:"NAMESPACE"`
	Service   string         `required:"true" envconfig:"SERVICE"`
	IdleAfter *time.Duration `envconfig:"IDLE_AFTER"`
}

// Parse extracts the configuration defined by Environment variables
func Parse() (*Spec, error) {
	s := &Spec{}
	err := envconfig.Process("proxy", s)
	if err != nil {
		return nil, err
	}

	if s.IdleAfter == nil {
		ia := time.Duration(3 * time.Minute)
		s.IdleAfter = &ia
	}

	return s, nil
}
