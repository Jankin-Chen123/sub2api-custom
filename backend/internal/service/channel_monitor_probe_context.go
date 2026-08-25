package service

import "context"

type channelMonitorProbeContextKey struct{}

func withChannelMonitorProbe(ctx context.Context) context.Context {
	return context.WithValue(ctx, channelMonitorProbeContextKey{}, true)
}

func isChannelMonitorProbe(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(channelMonitorProbeContextKey{}).(bool)
	return value
}
