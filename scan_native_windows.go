//go:build windows

package main

import "context"

func scanNative(ctx context.Context, cfg config) (report, bool, error) {
	rep, err := scanWindows(ctx, cfg)
	return rep, true, err
}
