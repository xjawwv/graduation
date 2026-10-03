package api

import (
	"context"
	"strconv"
)

type adminContextKey struct{}

func withAdminID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, adminContextKey{}, id)
}
func adminID(ctx context.Context) uint64 { id, _ := ctx.Value(adminContextKey{}).(uint64); return id }
func parseUint(v string) (uint64, error) { return strconv.ParseUint(v, 10, 64) }
