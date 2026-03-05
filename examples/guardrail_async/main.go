package main

import (
	"context"
	"strings"
	"time"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

func main() {
	mgr := agentsdk.NewGuardrailManager(true)
	mgr.AddInputV2("moderation_api", func(ctx context.Context, gCtx *agentsdk.GuardrailContext) (*agentsdk.GuardrailResultData, error) {
		// 假设这里调用外部内容安全服务；ctx 可被上游取消。
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = reqCtx

		if strings.Contains(strings.ToLower(gCtx.Text), "hack") {
			return &agentsdk.GuardrailResultData{
				Passed: false,
				Reason: "unsafe content",
			}, nil
		}
		return &agentsdk.GuardrailResultData{Passed: true}, nil
	})

	_ = mgr.CheckInputWithContext(context.Background(), "hello world", nil, nil)
}
