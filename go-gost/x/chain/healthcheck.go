package chain

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	corechain "github.com/go-gost/core/chain"
	"github.com/go-gost/core/selector"
)

const (
	healthCheckURL      = "http://cp.cloudflare.com/generate_204"
	healthCheckInterval = 30 * time.Second
)

// healthCheck periodically checks each chain and marks failures.
func (p *chainGroup) healthCheck() {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		for _, ch := range p.chains {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := probeChain(ctx, ch)
			cancel()
			if m, ok := ch.(selector.Markable); ok {
				if marker := m.Marker(); marker != nil {
					if err != nil {
						marker.Mark()
					} else {
						marker.Reset()
					}
				}
			}
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