package chain

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	corechain "github.com/go-gost/core/chain"
	"github.com/go-gost/core/logger"
	"github.com/go-gost/core/selector"
)

const (
	healthCheckURL      = "http://cp.cloudflare.com/generate_204"
	healthCheckInterval = 30 * time.Second
)

// healthCheck periodically checks each chain and marks failures.
func (p *chainGroup) healthCheck() {
	log := logger.Default().WithFields(map[string]any{"kind": "health-check"})
	p.checkChains(log)
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		p.checkChains(log)
	}
}

func (p *chainGroup) checkChains(log logger.Logger) {
	for _, ch := range p.chains {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := probeChain(ctx, ch)
		cancel()

		name := ""
		if cn, ok := ch.(chainNamer); ok {
			name = cn.Name()
		}

		if m, ok := ch.(selector.Markable); ok {
			if marker := m.Marker(); marker != nil {
				if err != nil {
					log.Warnf("chain %s health check failed: %v", name, err)
					marker.Mark()
				} else {
					if marker.Count() > 0 {
						log.Infof("chain %s back to normal", name)
					}
					marker.Reset()
				}
				continue
			}
		}
		if err != nil {
			log.Warnf("chain %s health check failed: %v", name, err)
		}
	}
}

func probeChain(ctx context.Context, c corechain.Chainer) error {
	r := NewRouter(
		corechain.ChainRouterOption(c),
		corechain.TimeoutRouterOption(10*time.Second),
	)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return r.Dial(ctx, network, addr)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	resp, err := client.Get(healthCheckURL)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}