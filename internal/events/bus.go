// Package events provides a lightweight in-process event bus for AOM lifecycle events.
// CLI commands emit events; subscribers (hook runner, notifiers) react to them.
// Async subscribers run in a goroutine and never block the caller.
// Sync subscribers run inline and may return an error that blocks the operation.
package events

import "fmt"

// Type constants for well-known AOM lifecycle events.
const (
	TaskCreated         = "task.created"
	TaskDone            = "task.done"
	TaskReady           = "task.ready"
	TaskBlocked         = "task.blocked"
	TaskNeedsAttention  = "task.needs-attention"
	TaskApprovalNeeded  = "task.approval-needed"
	TaskPlanProposed    = "task.plan-proposed"
	TaskPlanApproved    = "task.plan-approved"
	TaskPlanRejected    = "task.plan-rejected"
	SessionSpawned      = "session.spawned"
	SessionIdle         = "session.idle"
)

// Event is a lifecycle signal emitted when AOM state changes.
type Event struct {
	Type      string // one of the Type constants above
	RepoPath  string
	TaskID    string
	AgentName string
	Title     string
	Status    string
}

// AsyncHandler is called in a goroutine; errors are silently dropped.
type AsyncHandler func(Event)

// SyncHandler is called inline; a non-nil error blocks the originating operation.
type SyncHandler func(Event) error

// Bus dispatches events to registered handlers.
// Zero value is valid and safe to use (no subscribers, all emissions are no-ops).
type Bus struct {
	async []AsyncHandler
	sync  []SyncHandler
}

// SubscribeAsync registers a handler that runs in a goroutine and never blocks.
func (b *Bus) SubscribeAsync(h AsyncHandler) {
	b.async = append(b.async, h)
}

// SubscribeSync registers a handler that runs inline before the command proceeds.
// If the handler returns an error, Emit returns that error and the operation is blocked.
func (b *Bus) SubscribeSync(h SyncHandler) {
	b.sync = append(b.sync, h)
}

// Emit dispatches the event to all subscribers. Sync handlers run first; if any
// returns an error, async handlers are skipped and the error is returned.
// Async handlers fire in background goroutines and never block the caller.
func (b *Bus) Emit(e Event) error {
	for _, h := range b.sync {
		if err := h(e); err != nil {
			return fmt.Errorf("event %q blocked by hook: %w", e.Type, err)
		}
	}
	for _, h := range b.async {
		h := h
		go h(e)
	}
	return nil
}
