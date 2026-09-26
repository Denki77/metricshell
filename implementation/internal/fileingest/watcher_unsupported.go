//go:build !linux

package fileingest

import (
	"context"
	"time"
)

// Run reconciles periodically on platforms without Linux inotify support.
// Production artifacts target Linux; this fallback keeps the package usable by
// host-side tooling and preserves the transport's polling semantics.
func (reconciler *Reconciler) Run(ctx context.Context) error {
	reconciler.Reconcile(ctx, Startup)
	ticker := time.NewTicker(reconciler.configuration.ReconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			reconciler.Reconcile(ctx, Periodic)
		}
	}
}
