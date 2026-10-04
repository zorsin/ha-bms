package service

import (
	"sync"
	"time"
)

// StateUpdate represents the data payload from your WebSocket.
type StateUpdate struct {
	ID    string
	Value string
}

type BroadcastSubscribers map[chan StateUpdate]struct{}

const duplicateSuppressionWindow = time.Second

type broadcastValue struct {
	value     string
	published time.Time
}

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]BroadcastSubscribers
	lastValues  map[string]broadcastValue
	latest      map[string]StateUpdate
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[string]BroadcastSubscribers),
		lastValues:  make(map[string]broadcastValue),
		latest:      make(map[string]StateUpdate),
	}
}

func (b *Broadcaster) Subscribe(id string, bufferSize int) chan StateUpdate {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan StateUpdate, bufferSize)
	if _, ok := b.subscribers[id]; !ok {
		b.subscribers[id] = make(BroadcastSubscribers)
	}
	b.subscribers[id][ch] = struct{}{}
	if update, ok := b.latest[id]; ok {
		ch <- update
	}
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan StateUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, subs := range b.subscribers {
		if _, ok := subs[ch]; ok {
			delete(subs, ch)
			close(ch)
			if len(subs) == 0 {
				delete(b.subscribers, id)
			}
			break
		}
	}
}

func (b *Broadcaster) Publish(update StateUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.latest[update.ID] = update

	if subs, ok := b.subscribers[update.ID]; ok {
		now := time.Now()
		if previous, ok := b.lastValues[update.ID]; ok && previous.value == update.Value && now.Sub(previous.published) < duplicateSuppressionWindow {
			return
		}
		b.lastValues[update.ID] = broadcastValue{value: update.Value, published: now}
		for ch := range subs {
			select {
			case ch <- update:
				// default:
				// 	// Buffer full: drop or handle backpressure to avoid blocking WS reader
				// 	fmt.Println("[Warning] Dropped update for slow subscriber")
			}
		}
	}
}
