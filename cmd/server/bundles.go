package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// approving guards against double taps publishing the same bundle twice.
var approving sync.Map

// A Bundle is one daily post produced by the generator: 2-3 drafts sharing a bundle_id.
// The operator only approves (→ carousel on Instagram) or asks for a regeneration.
type Bundle struct {
	ID        string    `json:"id"`
	Caption   string    `json:"caption"`
	StyleID   string    `json:"style_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Drafts    []Draft   `json:"drafts"`
}

func (s *Server) bundles() []Bundle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byID := map[string]*Bundle{}
	var order []string
	for _, d := range s.drafts {
		if d.BundleID == "" {
			continue
		}
		b, ok := byID[d.BundleID]
		if !ok {
			b = &Bundle{ID: d.BundleID, Caption: d.Caption, StyleID: d.StyleID, CreatedAt: d.CreatedAt}
			byID[d.BundleID] = b
			order = append(order, d.BundleID)
		}
		if d.CreatedAt.After(b.CreatedAt) {
			b.CreatedAt = d.CreatedAt
		}
		b.Drafts = append(b.Drafts, d)
	}
	out := make([]Bundle, 0, len(order))
	for _, id := range order {
		b := byID[id]
		sort.SliceStable(b.Drafts, func(i, j int) bool { return b.Drafts[i].Order < b.Drafts[j].Order })
		out = append(out, *b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID > out[j].ID }) // newest day first
	return out
}

func (s *Server) handleGetBundles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.bundles())
}

type bundleRequest struct {
	BundleID string `json:"bundle_id"`
	Caption  string `json:"caption,omitempty"`
}

func (s *Server) findBundle(w http.ResponseWriter, r *http.Request) (*Bundle, *bundleRequest, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, nil, false
	}
	var req bundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BundleID == "" {
		http.Error(w, `{"error":"bundle_id required"}`, http.StatusBadRequest)
		return nil, nil, false
	}
	for _, b := range s.bundles() {
		if b.ID == req.BundleID {
			return &b, &req, true
		}
	}
	http.Error(w, `{"error":"bundle not found"}`, http.StatusNotFound)
	return nil, nil, false
}

// handleApproveBundle publishes the bundle as a carousel and tells the generator it was approved
// (so a freshly composed face can be promoted into the muse pool).
func (s *Server) handleApproveBundle(w http.ResponseWriter, r *http.Request) {
	b, req, ok := s.findBundle(w, r)
	if !ok {
		return
	}
	if _, busy := approving.LoadOrStore(b.ID, true); busy {
		http.Error(w, `{"error":"already publishing"}`, http.StatusConflict)
		return
	}
	defer approving.Delete(b.ID)
	caption := req.Caption
	if caption == "" {
		caption = b.Caption
	}
	targets := make([]*Draft, len(b.Drafts))
	for i := range b.Drafts {
		targets[i] = &b.Drafts[i]
	}
	mediaID, err := s.publishDrafts(targets, caption)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	s.sseBroker.BroadcastJob(GenerateJob{ID: b.ID, Type: "approved", RecipeID: b.ID})
	json.NewEncoder(w).Encode(map[string]string{"status": "published", "media_id": mediaID})
}

// handleRegenerateBundle discards the bundle and asks the generator to render that day again.
func (s *Server) handleRegenerateBundle(w http.ResponseWriter, r *http.Request) {
	b, _, ok := s.findBundle(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	var keep []Draft
	for _, d := range s.drafts {
		if d.BundleID == b.ID {
			_ = os.Remove(filepath.Join(s.mediaDir, d.FileName))
			continue
		}
		keep = append(keep, d)
	}
	s.drafts = keep
	s.saveDrafts()
	s.mu.Unlock()

	s.sseBroker.BroadcastJob(GenerateJob{ID: b.ID, Type: "regenerate", RecipeID: b.ID})
	log.Printf("🔁 Regeneration requested for %s", b.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "regenerating"})
}
