package control

import (
	"context"
	"net/netip"
	"net/url"
	"strings"

	"reasonix/internal/contract/config"
	"reasonix/internal/contract/provider"
	"reasonix/internal/runtime/agent"
	"reasonix/internal/state/sessionstore"
)

type foregroundTurnEnd struct {
	marker    sessionstore.InFlightTurnMeta
	live      *agent.LiveCompactTurn
	completed bool
}

func (e *foregroundTurnEnd) begin(c *Controller, ctx context.Context) context.Context {
	if e.marker.ID != "" {
		e.live = c.beginLiveCompact()
	}
	return e.live.Context(ctx)
}

func (e *foregroundTurnEnd) finish(c *Controller, ctx context.Context, start int, err error) {
	var version uint64
	if e.completed && err == nil && e.live.Answered() {
		version = c.executor.Session().TranscriptVersion()
	}
	if c.finishInFlightTurn(start, e.marker) && version != 0 && ctx.Err() == nil && !c.CancelRequested() {
		c.finishLiveCompact(ctx, e.live, version)
	}
}

func (c *Controller) beginLiveCompact() *agent.LiveCompactTurn {
	if c.executor == nil {
		return nil
	}
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	return c.executor.BeginLiveCompact(c.SessionPath(), c.liveCompactCloud)
}

func (c *Controller) finishLiveCompact(ctx context.Context, turn *agent.LiveCompactTurn, version uint64) {
	allow := func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return !c.gate.canceling && !c.gate.closed && c.sessionPath == turn.Path()
	}
	turn.Finish(ctx, version, func(publish func()) {
		c.snapshotMu.Lock()
		defer c.snapshotMu.Unlock()
		if allow() {
			publish()
		}
	}, allow)
}

func liveCompactCloud(entry *config.ProviderEntry) bool {
	if entry == nil {
		return false
	}
	if endpoint := strings.TrimSpace(entry.RequestURL); endpoint != "" && !provider.EndpointOverrideRepeatsBase(endpoint, entry.BaseURL) {
		return publicLiveCompactEndpoint(endpoint)
	}
	if wire, _ := config.ProtocolFor(entry.Kind); wire.Kind == "openai" {
		if endpoint := strings.TrimSpace(entry.ChatURL); endpoint != "" && !provider.EndpointOverrideRepeatsBase(endpoint, entry.BaseURL) {
			return publicLiveCompactEndpoint(endpoint)
		}
	}
	return publicLiveCompactEndpoint(strings.TrimSpace(entry.BaseURL))
}

func publicLiveCompactEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		return ip.IsGlobalUnicast() && !ip.IsPrivate() && !netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
	}
	return strings.Contains(host, ".") && !strings.HasSuffix(host, ".local") && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".internal")
}
