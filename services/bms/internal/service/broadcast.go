package service

import (
	"fmt"
	"sync"
)

// StateUpdate represents the data payload from your WebSocket.
type StateUpdate struct {
	ID    string
	Value string
}

type BroadcastSubscribers map[chan StateUpdate]struct{}

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]BroadcastSubscribers
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[string]BroadcastSubscribers),
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
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, ok := b.subscribers[update.ID]; ok {
		for ch := range subs {
			select {
			case ch <- update:
			default:
				// Buffer full: drop or handle backpressure to avoid blocking WS reader
				fmt.Println("[Warning] Dropped update for slow subscriber")
			}
		}
	}
}
