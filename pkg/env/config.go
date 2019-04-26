package env

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Spec hold configuration of the proxy to build
type Spec struct {
	Namespace  string         `required:"true" envconfig:"NAMESPACE"`
	Deployment string         `required:"true" envconfig:"DEPLOYMENT"`
	Service    string         `required:"true" envconfig:"SERVICE"`
	IdleAfter  *time.Duration `envconfig:"IDLE_AFTER"`
}

// Parse extracts the configuration defined by Environment variables
func Parse() (*Spec, error) {
	s := &Spec{}
	err := envconfig.Process("proxy", s)
	if err != nil {
		return nil, err
	}

	if s.IdleAfter == nil {
		ia := time.Duration(90 * time.Second)
		s.IdleAfter = &ia
	}

	return s, nil
}
