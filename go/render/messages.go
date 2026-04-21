package render

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Message represents a single on-screen message in the right panel.
type Message struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Color     string    `json:"color"` // hex RGB, e.g. "ff5500"
	Timestamp time.Time `json:"timestamp"`
	TTL       int       `json:"ttl"` // seconds until auto-dismiss; 0 = never
}

// MessageQueue manages the scrollable message list.
type MessageQueue struct {
	mu       sync.RWMutex
	messages []Message
	maxLen   int
}

const maxMessages = 20

// NewMessageQueue creates a queue with a maximum length.
func NewMessageQueue() *MessageQueue {
	return &MessageQueue{maxLen: maxMessages}
}

func randomID(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

// Add appends a message. Evicts oldest if at capacity.
func (q *MessageQueue) Add(text string, color string, ttl int) Message {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(text) > 200 {
		text = text[:200]
	}
	if color == "" {
		color = "ffffff"
	}

	msg := Message{
		ID:        randomID(8),
		Text:      text,
		Color:     color,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	q.messages = append(q.messages, msg)
	if len(q.messages) > q.maxLen {
		q.messages = q.messages[1:]
	}
	return msg
}

// Remove deletes a message by ID. Returns true if found.
func (q *MessageQueue) Remove(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, m := range q.messages {
		if m.ID == id {
			q.messages = append(q.messages[:i], q.messages[i+1:]...)
			return true
		}
	}
	return false
}

// List returns a snapshot of all messages.
func (q *MessageQueue) List() []Message {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]Message, len(q.messages))
	copy(out, q.messages)
	return out
}

// Expire removes messages whose TTL has elapsed.
func (q *MessageQueue) Expire() {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	keep := make([]Message, 0, len(q.messages))
	for _, m := range q.messages {
		if m.TTL > 0 && now.Sub(m.Timestamp) > time.Duration(m.TTL)*time.Second {
			continue
		}
		keep = append(keep, m)
	}
	q.messages = keep
}

// DrawRightPanel renders the message queue onto the message panel area.
func (q *MessageQueue) DrawRightPanel(d *Dashboard) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	px, py, pw, ph := d.MessagePanelBounds()
	// Divider: horizontal for portrait, vertical for landscape
	if d.Orientation == OrientationPortrait || d.Orientation == OrientationReversePortrait {
		d.DrawHorizontalDivider(py, 0, d.W, 60, 60, 80)
	} else {
		d.DrawDivider(px, py, py+ph, 60, 60, 80)
	}

	textMaxW := pw - PanelPaddingX*2 - 6 // padding on both sides + border/gap
	y := py + MsgStartY
	for _, m := range q.messages {
		lines := d.WrapText(m.Text, textMaxW)
		msgH := len(lines) * MsgLineH
		if y+msgH > py+ph {
			break
		}
		r, g, b := hexToRGB(m.Color)
		d.DrawMessageBox(px+PanelPaddingX, y, MsgLineH, lines, m.Color, r, g, b)
		y += msgH
	}
}
