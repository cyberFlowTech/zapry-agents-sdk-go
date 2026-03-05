package main

import (
	"context"
	"time"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

func main() {
	scheduler := agentsdk.NewProactiveScheduler(time.Minute, func(userID string, text string) error {
		return nil
	}, nil)

	rt := &agentsdk.SDKRuntime{
		ProactiveScheduler: scheduler,
		MemoryStore:        agentsdk.NewInMemoryMemoryStore(),
		ShutdownHooks: []agentsdk.ShutdownFunc{
			func(ctx context.Context) error {
				// 这里可以放 tracing flush、metrics flush 等自定义逻辑。
				return nil
			},
		},
	}

	_ = rt.Shutdown(context.Background())
}
