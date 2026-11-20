//go:build !windows

package main

import "context"

func scanNative(context.Context, config) (report, bool, error) {
	return report{}, false, nil
}
