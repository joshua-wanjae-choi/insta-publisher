package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type GenerateJob struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"` // "initial", "refine", "variations"
	StyleID        string   `json:"style_id"`
	StyleName      string   `json:"style_name"`
	Custom         string   `json:"custom_prompt,omitempty"`
	BaseImageURL   string   `json:"base_image_url,omitempty"`
	Feedback       string   `json:"feedback,omitempty"`
	PresetAngles   []string `json:"preset_angles,omitempty"`
	CandidateCount int      `json:"candidate_count,omitempty"`
}

type SSEBroker struct {
	mu      sync.Mutex
	clients map[chan string]bool
}

func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan string]bool),
	}
}

func (b *SSEBroker) AddClient(ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[ch] = true
}

func (b *SSEBroker) RemoveClient(ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *SSEBroker) BroadcastJob(job GenerateJob) {
	data, _ := json.Marshal(job)
	msg := fmt.Sprintf("event: generate\ndata: %s\n\n", string(data))

	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
			// 버퍼 찼을 경우 스킵
		}
	}
}

func (s *Server) handleAgentStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Caddy / Nginx 버퍼링 방지

	msgChan := make(chan string, 10)
	s.sseBroker.AddClient(msgChan)
	defer s.sseBroker.RemoveClient(msgChan)

	// 초기 연결 신호 전송
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ready\"}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			// Caddy 프록시 유휴 타임아웃 방지용 하트비트 핑
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case msg := <-msgChan:
			fmt.Fprint(w, msg)
			flusher.Flush()
		}
	}
}
