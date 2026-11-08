package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"insta/pkg/config"
	"insta/pkg/igclient"
)

//go:embed web/static/*
var staticFS embed.FS

type Draft struct {
	ID         string    `json:"id"`
	FileName   string    `json:"file_name"`
	PreviewURL string    `json:"preview_url"`
	Caption    string    `json:"caption"`
	StyleID    string    `json:"style_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Server struct {
	cfg            *config.Config
	client         *igclient.Client
	adminUser      string
	adminPass      string
	mfaSecret      string
	uploadToken    string
	baseURL        string
	mediaDir       string
	draftsFile     string
	guard          *SecurityGuard
	sseBroker      *SSEBroker
	styleStore     *StyleStore
	mu             sync.RWMutex
	drafts         []Draft
	sessionsMu     sync.RWMutex
	activeSessions map[string]time.Time
}

func main() {
	cfg, err := config.Load(".env")
	if err != nil {
		log.Printf("Notice: could not load .env: %v", err)
	}

	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := os.Getenv("ADMIN_PASS")
	if adminPass == "" {
		adminPass = "admin1234!"
	}
	mfaSecret := os.Getenv("MFA_SECRET")
	uploadToken := os.Getenv("UPLOAD_API_TOKEN")
	if uploadToken == "" {
		uploadToken = "gen_secret_token_123"
	}

	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	mediaDir := "media"
	_ = os.MkdirAll(mediaDir, 0755)

	client := igclient.New(cfg.InstagramAccountID, cfg.AccessToken, cfg.APIVersion)

	srv := &Server{
		cfg:            cfg,
		client:         client,
		adminUser:      adminUser,
		adminPass:      adminPass,
		mfaSecret:      mfaSecret,
		uploadToken:    uploadToken,
		baseURL:        baseURL,
		mediaDir:       mediaDir,
		draftsFile:     "drafts.json",
		guard:          NewSecurityGuard(),
		sseBroker:      NewSSEBroker(),
		styleStore:     NewStyleStore("styles.json"),
		activeSessions: make(map[string]time.Time),
	}
	srv.loadDrafts()

	mux := http.NewServeMux()

	// 1. Static Web UI
	mux.HandleFunc("/", srv.handleUI)

	// 2. Public Media File Serving (인스타그램 크롤러 접근 허용하되, 디렉토리 인덱싱 브라우징 차단)
	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(r.URL.Path, "/media/")
		if relPath == "" || strings.HasSuffix(relPath, "/") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		// 경로 순회(Path traversal) 방지
		cleanPath := filepath.Clean(relPath)
		if strings.Contains(cleanPath, "..") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
		fullPath := filepath.Join(mediaDir, cleanPath)
		fi, err := os.Stat(fullPath)
		if err != nil || fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, fullPath)
	})

	// 3. Auth
	mux.HandleFunc("/api/login", srv.handleLogin)
	mux.HandleFunc("/api/logout", srv.handleLogout)

	// 4. 맥미니 연동 전용 SSE & Upload (Token 필요)
	mux.HandleFunc("/api/agent/stream", srv.tokenAuth(srv.handleAgentStream))
	mux.HandleFunc("/api/upload", srv.tokenAuth(srv.handleUpload))

	// 5. 웹 대시보드 API (Login 세션 필요)
	mux.HandleFunc("/api/styles", srv.webAuth(srv.handleGetStyles))
	mux.HandleFunc("/api/generate-request", srv.webAuth(srv.handleGenerateRequest))
	mux.HandleFunc("/api/drafts", srv.webAuth(srv.handleGetDrafts))
	mux.HandleFunc("/api/drafts/", srv.webAuth(srv.handleDeleteDraft))
	mux.HandleFunc("/api/publish", srv.webAuth(srv.handlePublish))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Fashion Insta-Publisher running on port %s", port)
	log.Printf("🌐 Public Base URL: %s", baseURL)

	// Start background token refresher (extends 60-day token every 15 days automatically)
	srv.startTokenRefresher()

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}

// ----------------- Auth & Security -----------------

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if blocked, remaining := s.guard.CheckBlocked(r); blocked {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"error": fmt.Sprintf("로그인 5회 실패로 접속이 일시 차단되었습니다. %d분 후 다시 시도하세요.", int(remaining.Minutes())+1),
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	userMatch := SafeCompare(req.Username, s.adminUser)
	passMatch := SafeCompare(req.Password, s.adminPass)

	if !userMatch || !passMatch {
		s.guard.RecordFailure(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "아이디 또는 비밀번호가 올바르지 않습니다."})
		return
	}

	// 🔐 MFA 활성화 여부 확인
	if s.mfaSecret != "" {
		if req.MFACode == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted) // 202 Accepted -> 프론트엔드에 OTP 입력 요구
			json.NewEncoder(w).Encode(map[string]any{
				"status":       "mfa_required",
				"mfa_required": true,
				"message":      "구글 OTP 6자리 인증번호를 입력하세요.",
			})
			return
		}

		if !ValidateTOTP(s.mfaSecret, req.MFACode) {
			s.guard.RecordFailure(r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "구글 OTP 인증번호가 올바르지 않습니다."})
			return
		}
	}

	s.guard.RecordSuccess(r)
	sessionToken := generateRandomToken(32)
	s.sessionsMu.Lock()
	s.activeSessions[sessionToken] = time.Now().Add(30 * 24 * time.Hour)
	s.sessionsMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "insta_session",
		Value:    sessionToken,
		Path:     "/",
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("insta_session")
	if err == nil {
		s.sessionsMu.Lock()
		delete(s.activeSessions, cookie.Value)
		s.sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "insta_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
	w.Write([]byte(`{"status":"logged_out"}`))
}

func (s *Server) isValidSession(r *http.Request) bool {
	cookie, err := r.Cookie("insta_session")
	if err != nil {
		return false
	}
	s.sessionsMu.RLock()
	expiry, exists := s.activeSessions[cookie.Value]
	s.sessionsMu.RUnlock()
	return exists && time.Now().Before(expiry)
}

func (s *Server) webAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.isValidSession(r) {
			next(w, r)
			return
		}
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
	}
}

func (s *Server) tokenAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		var token string
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			token = r.Header.Get("X-Upload-Token")
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if !SafeCompare(token, s.uploadToken) {
			http.Error(w, `{"error":"Unauthorized API Token"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func generateRandomToken(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ----------------- Handlers -----------------

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "index.html" {
		data, err := staticFS.ReadFile("web/static/index.html")
		if err != nil {
			http.Error(w, "UI not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}

	// Serve assets and static files
	filePath := filepath.Join("web/static", filepath.Clean(path))
	data, err := staticFS.ReadFile(filePath)
	if err != nil {
		// SPA fallback to index.html
		data, err = staticFS.ReadFile("web/static/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}

	if strings.HasSuffix(path, ".js") {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	} else if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else if strings.HasSuffix(path, ".png") {
		w.Header().Set("Content-Type", "image/png")
	} else if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
		w.Header().Set("Content-Type", "image/jpeg")
	} else if strings.HasSuffix(path, ".svg") {
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	w.Write(data)
}

// 사용자가 웹에서 사조를 선택하고 맥미니로 주문 쏠 때
func (s *Server) handleGenerateRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job GenerateJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	job.ID = fmt.Sprintf("job_%d", time.Now().UnixNano())
	log.Printf("📢 Broadcasting job to Mac Mini: Style=%s (ID=%s)", job.StyleName, job.ID)
	s.sseBroker.BroadcastJob(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "queued", "job_id": job.ID})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_ = r.ParseMultipartForm(32 << 20)
	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"image file required: %v"}`, err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	caption := r.FormValue("caption")
	styleID := r.FormValue("style_id")

	ext := strings.ToLower(filepath.Ext(handler.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		http.Error(w, `{"error":"only JPG/PNG allowed"}`, http.StatusBadRequest)
		return
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	fileName := fmt.Sprintf("%s%s", id, ext)
	dstPath := filepath.Join(s.mediaDir, fileName)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}

	draft := Draft{
		ID:         id,
		FileName:   fileName,
		PreviewURL: fmt.Sprintf("%s/media/%s", s.baseURL, fileName),
		Caption:    caption,
		StyleID:    styleID,
		CreatedAt:  time.Now(),
	}

	s.mu.Lock()
	s.drafts = append([]Draft{draft}, s.drafts...)
	s.saveDrafts()
	s.mu.Unlock()

	log.Printf("📥 Received draft from Mac Mini: %s", fileName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(draft)
}

func (s *Server) handleGetDrafts(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	if s.drafts == nil {
		json.NewEncoder(w).Encode([]Draft{})
		return
	}
	json.NewEncoder(w).Encode(s.drafts)
}

func (s *Server) handleDeleteDraft(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/drafts/")
	s.mu.Lock()
	defer s.mu.Unlock()

	var newDrafts []Draft
	var deleted *Draft
	for _, d := range s.drafts {
		if d.ID == id {
			deleted = &d
		} else {
			newDrafts = append(newDrafts, d)
		}
	}

	if deleted != nil {
		_ = os.Remove(filepath.Join(s.mediaDir, deleted.FileName))
		s.drafts = newDrafts
		s.saveDrafts()
		w.Write([]byte(`{"status":"deleted"}`))
		return
	}

	http.Error(w, "Draft not found", http.StatusNotFound)
}

type PublishRequest struct {
	ID      string   `json:"id"`
	IDs     []string `json:"ids,omitempty"`
	Caption string   `json:"caption"`
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	targetIDs := req.IDs
	if len(targetIDs) == 0 && req.ID != "" {
		targetIDs = []string{req.ID}
	}
	if len(targetIDs) == 0 {
		http.Error(w, `{"error":"no draft ids provided"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	var targets []*Draft
	for _, tid := range targetIDs {
		for _, d := range s.drafts {
			if d.ID == tid {
				copyD := d
				targets = append(targets, &copyD)
				break
			}
		}
	}
	s.mu.RUnlock()

	if len(targets) == 0 {
		http.Error(w, `{"error":"draft not found"}`, http.StatusNotFound)
		return
	}

	caption := req.Caption
	if caption == "" {
		caption = targets[0].Caption
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var mediaID string
	var err error

	if len(targets) == 1 {
		publicImageURL := fmt.Sprintf("%s/media/%s", s.baseURL, targets[0].FileName)
		log.Printf("Publishing single photo to Instagram: %s", publicImageURL)
		mediaID, err = s.client.UploadAndPublishPhoto(ctx, publicImageURL, caption)
	} else {
		var publicImageURLs []string
		for _, t := range targets {
			publicImageURLs = append(publicImageURLs, fmt.Sprintf("%s/media/%s", s.baseURL, t.FileName))
		}
		log.Printf("Publishing %d carousel photos to Instagram: %v", len(publicImageURLs), publicImageURLs)
		mediaID, err = s.client.UploadAndPublishCarousel(ctx, publicImageURLs, caption)
	}

	if err != nil {
		log.Printf("Instagram publish error: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	log.Printf("🎉 Instagram published successfully! Media ID: %s", mediaID)

	for _, t := range targets {
		if t.StyleID != "" {
			s.styleStore.MarkPublished(t.StyleID)
		}
		_ = os.Remove(filepath.Join(s.mediaDir, t.FileName))
	}

	targetSet := make(map[string]bool)
	for _, t := range targets {
		targetSet[t.ID] = true
	}

	s.mu.Lock()
	var newDrafts []Draft
	for _, d := range s.drafts {
		if !targetSet[d.ID] {
			newDrafts = append(newDrafts, d)
		}
	}
	s.drafts = newDrafts
	s.saveDrafts()
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "published",
		"media_id": mediaID,
	})
}

func (s *Server) loadDrafts() {
	data, err := os.ReadFile(s.draftsFile)
	if err == nil {
		_ = json.Unmarshal(data, &s.drafts)
	}
}

func (s *Server) saveDrafts() {
	data, _ := json.MarshalIndent(s.drafts, "", "  ")
	_ = os.WriteFile(s.draftsFile, data, 0644)
}

// startTokenRefresher runs a periodic worker every 15 days to extend the 60-day token
func (s *Server) startTokenRefresher() {
	ticker := time.NewTicker(15 * 24 * time.Hour)
	go func() {
		for range ticker.C {
			log.Println("🔄 [TokenRefresher] Attempting periodic Instagram token refresh...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			newToken, err := s.client.RefreshToken(ctx)
			cancel()
			if err != nil {
				log.Printf("⚠️ [TokenRefresher] Failed to refresh token: %v", err)
				continue
			}

			log.Println("✅ [TokenRefresher] Token successfully extended for another 60 days!")
			s.updateEnvFile("GRAPH_API_ACCESS_TOKEN", newToken)
			s.updateEnvFile("INSTAGRAM_ACCESS_TOKEN", newToken)
		}
	}()
}

func (s *Server) updateEnvFile(key, val string) {
	envPath := ".env"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, val)
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, val))
	}

	_ = os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
}
