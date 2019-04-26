package metrics

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	metricPort = 19999
)

var log = logf.Log.WithName("controller").WithName("metrics")

type Collector struct {
	stats *Proxy

	mu *sync.RWMutex
}

func NewCollector() *Collector {
	return &Collector{
		mu: &sync.RWMutex{},
	}
}

func (c *Collector) CurrentStats() *Proxy {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stats == nil {
		return &Proxy{}
	}

	return c.stats
}

func (c *Collector) Start(stopCh <-chan struct{}) {
	t := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-t.C:
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
	data, err := requestStats(fmt.Sprintf("http://localhost:19999/metrics"))
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
	tr := &http.Transport{
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   5 * time.Second,
	}

	res, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}
