package mongo

import (
	"context"
	"strconv"
	"sync"

	"go.mongodb.org/mongo-driver/v2/event"
)

type monitorBuilder struct {
	startedCBs   []func(ctx context.Context, startedEvent *event.CommandStartedEvent)
	succeededCBs []func(ctx context.Context, startedEvent *event.CommandStartedEvent, succeededEvent *event.CommandSucceededEvent)
	failedCBs    []func(ctx context.Context, startedEvent *event.CommandStartedEvent, failedEvent *event.CommandFailedEvent)
}

func (b *monitorBuilder) Add(
	startedCb func(ctx context.Context, startedEvent *event.CommandStartedEvent),
	succeededCb func(ctx context.Context, startedEvent *event.CommandStartedEvent, succeededEvent *event.CommandSucceededEvent),
	failedCb func(ctx context.Context, startedEvent *event.CommandStartedEvent, failedEvent *event.CommandFailedEvent),
) {
	b.startedCBs = append(b.startedCBs, startedCb)
	b.succeededCBs = append(b.succeededCBs, succeededCb)
	b.failedCBs = append(b.failedCBs, failedCb)
}

func (b *monitorBuilder) Build() *event.CommandMonitor {
	pendingQueue := make(map[int64]*event.CommandStartedEvent)
	pendingQueueMu := sync.Mutex{}

	return &event.CommandMonitor{
		Started: func(ctx context.Context, startedEvent *event.CommandStartedEvent) {
			pendingQueueMu.Lock()
			pendingQueue[startedEvent.RequestID] = startedEvent
			pendingQueueMu.Unlock()

			for i := range b.startedCBs {
				b.startedCBs[i](ctx, startedEvent)
			}
		},
		Succeeded: func(ctx context.Context, succeededEvent *event.CommandSucceededEvent) {
			pendingQueueMu.Lock()
			startedEvent := pendingQueue[succeededEvent.RequestID]
			delete(pendingQueue, succeededEvent.RequestID)
			pendingQueueMu.Unlock()

			if startedEvent != nil {
				for i := range b.succeededCBs {
					b.succeededCBs[i](ctx, startedEvent, succeededEvent)
				}
			}
		},
		Failed: func(ctx context.Context, failedEvent *event.CommandFailedEvent) {
			pendingQueueMu.Lock()
			startedEvent := pendingQueue[failedEvent.RequestID]
			delete(pendingQueue, failedEvent.RequestID)
			pendingQueueMu.Unlock()

			if startedEvent != nil {
				for i := range b.failedCBs {
					b.failedCBs[i](ctx, startedEvent, failedEvent)
				}
			}
		},
	}
}

var operationsMap = map[string]interface{}{
	"insert":        nil,
	"find":          nil,
	"update":        nil,
	"delete":        nil,
	"findAndModify": nil,
}

func determineCollection(startedEvent *event.CommandStartedEvent) string {
	elements, _ := startedEvent.Command.Elements()
	for i := range elements {
		_, ok := operationsMap[elements[i].Key()]
		if ok {
			quoted := elements[i].Value().String()
			unquoted, err := strconv.Unquote(quoted)
			if err != nil {
				return quoted
			}

			return unquoted
		}
	}
	return ""
}
