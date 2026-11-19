package main

import (
	"context"
	"encoding/json"
	"os"

	extension "github.com/esengine/DeepSeek-Reasonix/sdk/go"
)

type liveCompact struct{}

func (liveCompact) Initialize(context.Context, extension.InitializeParams) (*extension.InitializeResult, error) {
	return &extension.InitializeResult{Subscriptions: []string{string(extension.EventCompactionLive)}}, nil
}

type offeredRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func selectCut(_ context.Context, _ string, raw json.RawMessage) (*extension.InterceptResult, error) {
	var p struct {
		Candidates []offeredRange `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || len(p.Candidates) == 0 {
		return extension.Continue(), nil
	}
	return extension.Replace(map[string]offeredRange{"cut": p.Candidates[0]})
}

func main() {
	err := extension.Serve(context.Background(), liveCompact{}, extension.Options{
		Name:    "live-compact",
		Version: "0.1.0",
		Interceptors: map[string]extension.InterceptorFunc{
			string(extension.EventCompactionLive): selectCut,
		},
	})
	if err != nil {
		os.Exit(1)
	}
}
