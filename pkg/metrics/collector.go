package metrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/gojektech/heimdall"
	"github.com/gojektech/heimdall/httpclient"
)

const (
	metricPort = 19999
)

var log = logf.Log.WithName("controller").WithName("metrics")

// Collector defines a metrics collector
type Collector struct {
	stats *Proxy

	mu *sync.RWMutex
}

// NewCollector returns a new Collector instance
func NewCollector() *Collector {
	return &Collector{
		mu: &sync.RWMutex{},
	}
}

// CurrentStats returns current stats
func (c *Collector) CurrentStats() *Proxy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stats == nil {
		return &Proxy{}
	}

	return c.stats
}

// Start ...
func (c *Collector) Start(stopCh <-chan struct{}) {
	for t := time.Tick(6 * time.Second); ; {
		select {
		case <-t:
			p, err := getMetrics()
			if err != nil {
				log.Error(err, "obtaining stats")
				continue
			}

			c.mu.Lock()
			c.stats = p
			c.mu.Unlock()
		case <-stopCh:
			break
		}
	}
}

func getMetrics() (*Proxy, error) {
	data, err := requestStats(fmt.Sprintf("http://127.0.0.1:%v/metrics", metricPort))
	if err != nil {
		return nil, err
	}

	p, err := parse(data)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func requestStats(url string) ([]byte, error) {
	httpClient := httpclient.NewClient(
		httpclient.WithHTTPTimeout(10*time.Second),
		httpclient.WithRetryCount(1),
		httpclient.WithRetrier(
			heimdall.NewRetrier(
				heimdall.NewConstantBackoff(10*time.Millisecond, 50*time.Millisecond),
			),
		),
	)

	headers := http.Header{}
	response, err := httpClient.Get(url, headers)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
