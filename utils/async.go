package utils

import (
	"context"
	"github.com/amazechain/amc/log"
	"reflect"
	"runtime"
	"time"
)

// RunEvery runs the provided command periodically.
// It runs in a goroutine, and can be cancelled by finishing the supplied context.
func RunEvery(ctx context.Context, period time.Duration, f func()) {
	funcName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	ticker := time.NewTicker(period)
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Trace("running", "function", funcName)
				f()
			case <-ctx.Done():
				log.Debug("context is closed, exiting", "function", funcName)
				ticker.Stop()
				return
			}
		}
	}()
}
