package handler

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
)

// RoomHub manages SSE subscriptions per room.
type RoomHub struct {
	mu    sync.RWMutex
	rooms map[string]map[chan string]bool // roomID → set of channels
}

func NewRoomHub() *RoomHub {
	return &RoomHub{rooms: make(map[string]map[chan string]bool)}
}

func (h *RoomHub) Subscribe(roomID string) chan string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[chan string]bool)
	}
	ch := make(chan string, 10)
	h.rooms[roomID][ch] = true
	return ch
}

func (h *RoomHub) Unsubscribe(roomID string, ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] != nil {
		delete(h.rooms[roomID], ch)
		close(ch)
	}
}

func (h *RoomHub) Broadcast(roomID string, message string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.rooms[roomID] {
		select {
		case ch <- message:
		default: // skip slow clients
		}
	}
}

// StreamRoom handles GET /api/rooms/:id/stream — SSE endpoint
func (rh *RoomsHandler) StreamRoom(c *gin.Context) {
	roomID := c.Param("id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := rh.hub.Subscribe(roomID)
	defer rh.hub.Unsubscribe(roomID, ch)

	ctx := c.Request.Context()
	c.Writer.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Writer.Flush()
		}
	}
}
