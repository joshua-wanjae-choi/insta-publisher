package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
)

type FashionStyle struct {
	ID        string `json:"id"`
	Era       string `json:"era,omitempty"`
	Name      string `json:"name"`
	Keywords  string `json:"keywords,omitempty"`
	Desc      string `json:"desc"`
	Published bool   `json:"published"`
}

type StyleStore struct {
	mu       sync.RWMutex
	filePath string
	styles   []FashionStyle
}

func NewStyleStore(filePath string) *StyleStore {
	store := &StyleStore{filePath: filePath}
	store.load()
	return store
}

func (s *StyleStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.filePath)
	if err == nil {
		_ = json.Unmarshal(data, &s.styles)
	}
}

func (s *StyleStore) save() {
	data, _ := json.MarshalIndent(s.styles, "", "  ")
	_ = os.WriteFile(s.filePath, data, 0644)
}

func (s *StyleStore) GetAll() []FashionStyle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.styles
}

func (s *StyleStore) MarkPublished(styleID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.styles {
		if s.styles[i].ID == styleID {
			s.styles[i].Published = true
			s.save()
			return
		}
	}
}

func (s *Server) handleGetStyles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.styleStore.GetAll())
}
