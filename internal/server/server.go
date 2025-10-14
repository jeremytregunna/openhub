package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jeremytregunna/openhub/internal/storage"
)

type Storage interface {
	CreateRepo(owner, name string) error
	DeleteRepo(owner, name string) error
	RepoExists(owner, name string) bool
	RepoPath(owner, name string) string
	ListRepos() ([]storage.Repo, error)
	ListReposByOwner(owner string) ([]storage.Repo, error)
	GetMetadata(owner, name string) (storage.Metadata, error)
	SetMetadata(owner, name string, meta storage.Metadata) error
}

type AuthStore interface {
	TokenValidator
	CreateUserWithToken(username, tokenName, token string) error
}

type Server struct {
	storage   Storage
	authStore AuthStore
	mux       *http.ServeMux
}

func New(storage Storage, authStore AuthStore) *Server {
	s := &Server{
		storage:   storage,
		authStore: authStore,
		mux:       http.NewServeMux(),
	}

	s.mux.HandleFunc("/api/repos/create", s.handleCreateRepo)
	s.mux.HandleFunc("/api/repos/delete", s.handleDeleteRepo)
	s.mux.HandleFunc("/api/repos/list", s.handleListRepos)
	s.mux.HandleFunc("/api/repos/metadata", s.handleMetadata)
	s.mux.HandleFunc("/api/repos/replicate", s.handleReplicate)
	s.mux.HandleFunc("/api/repos/register-replication", s.handleRegisterReplication)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type CreateRepoRequest struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type CreateRepoResponse struct {
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	RepoPath string `json:"repo_path,omitempty"`
	CloneURL string `json:"clone_url,omitempty"`
}

func (s *Server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Name == "" {
		s.jsonError(w, "owner and name required", http.StatusBadRequest)
		return
	}

	if !isValidName(req.Owner) || !isValidName(req.Name) {
		s.jsonError(w, "invalid owner or name", http.StatusBadRequest)
		return
	}

	if s.storage.RepoExists(req.Owner, req.Name) {
		s.jsonError(w, "repository already exists", http.StatusConflict)
		return
	}

	if err := s.storage.CreateRepo(req.Owner, req.Name); err != nil {
		s.jsonError(w, fmt.Sprintf("create failed: %v", err), http.StatusInternalServerError)
		return
	}

	resp := CreateRepoResponse{
		Success:  true,
		RepoPath: fmt.Sprintf("%s/%s.git", req.Owner, req.Name),
		CloneURL: fmt.Sprintf("http://localhost:3000/%s/%s.git", req.Owner, req.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(CreateRepoResponse{
		Success: false,
		Error:   msg,
	})
}

func (s *Server) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Name == "" {
		s.jsonError(w, "owner and name required", http.StatusBadRequest)
		return
	}

	if !s.storage.RepoExists(req.Owner, req.Name) {
		s.jsonError(w, "repository not found", http.StatusNotFound)
		return
	}

	if err := s.storage.DeleteRepo(req.Owner, req.Name); err != nil {
		s.jsonError(w, fmt.Sprintf("delete failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	owner := r.URL.Query().Get("owner")

	var repos []storage.Repo
	var err error

	if owner != "" {
		repos, err = s.storage.ListReposByOwner(owner)
	} else {
		repos, err = s.storage.ListRepos()
	}

	if err != nil {
		s.jsonError(w, fmt.Sprintf("list failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"repos":   repos,
	})
}

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	name := r.URL.Query().Get("name")

	if owner == "" || name == "" {
		s.jsonError(w, "owner and name required", http.StatusBadRequest)
		return
	}

	if !s.storage.RepoExists(owner, name) {
		s.jsonError(w, "repository not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		meta, err := s.storage.GetMetadata(owner, name)
		if err != nil {
			s.jsonError(w, fmt.Sprintf("get metadata failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"metadata": meta,
		})

	case "POST":
		var meta storage.Metadata
		if err := json.NewDecoder(r.Body).Decode(&meta); err != nil {
			s.jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.storage.SetMetadata(owner, name, meta); err != nil {
			s.jsonError(w, fmt.Sprintf("set metadata failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		s.jsonError(w, "missing authorization", http.StatusUnauthorized)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		s.jsonError(w, "invalid authorization header", http.StatusUnauthorized)
		return
	}

	username, err := s.authStore.ValidateAPIToken(parts[1])
	if err != nil {
		s.jsonError(w, "invalid token", http.StatusUnauthorized)
		return
	}

	var req struct {
		Owner         string          `json:"owner"`
		Repo          string          `json:"repo"`
		InstanceID    string          `json:"instance_id"`
		InvitationKey string          `json:"invitation_key"`
		Bundle        string          `json:"bundle"`
		Metadata      storage.Metadata `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Repo == "" || req.InstanceID == "" || req.InvitationKey == "" {
		s.jsonError(w, "owner, repo, instance_id, and invitation_key required", http.StatusBadRequest)
		return
	}

	if !isValidName(req.Owner) || !isValidName(req.Repo) {
		s.jsonError(w, "invalid owner or repo name", http.StatusBadRequest)
		return
	}

	if req.Bundle == "" {
		s.jsonError(w, "missing bundle data", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(username, "replication-") {
		s.jsonError(w, "unauthorized: not a replication user", http.StatusForbidden)
		return
	}

	expectedUser := fmt.Sprintf("replication-%s-%s-%s", req.Owner, req.Repo, req.InstanceID)
	if username != expectedUser {
		s.jsonError(w, "unauthorized: token mismatch", http.StatusForbidden)
		return
	}

	bundleData, err := base64.StdEncoding.DecodeString(req.Bundle)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("decode bundle: %v", err), http.StatusBadRequest)
		return
	}

	repoExists := s.storage.RepoExists(req.Owner, req.Repo)

	if repoExists {
		existingMeta, err := s.storage.GetMetadata(req.Owner, req.Repo)
		if err != nil {
			s.jsonError(w, fmt.Sprintf("get metadata failed: %v", err), http.StatusInternalServerError)
			return
		}

		if existingMeta.ReplicaOf == nil {
			s.jsonError(w, "cannot replicate: repo already exists as origin", http.StatusConflict)
			return
		}

		if existingMeta.ReplicaOf.InvitationKey != req.InvitationKey {
			s.jsonError(w, "invalid invitation key", http.StatusForbidden)
			return
		}
	} else {
		if err := s.storage.CreateRepo(req.Owner, req.Repo); err != nil {
			s.jsonError(w, fmt.Sprintf("create repo failed: %v", err), http.StatusInternalServerError)
			return
		}
	}

	repoPath := s.storage.RepoPath(req.Owner, req.Repo)
	bundlePath := filepath.Join(os.TempDir(), fmt.Sprintf("bundle-%s-%s.bundle", req.Owner, req.Repo))

	if err := os.WriteFile(bundlePath, bundleData, 0600); err != nil {
		s.jsonError(w, fmt.Sprintf("write bundle: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(bundlePath)

	cmd := exec.Command("git", "fetch", bundlePath, "refs/*:refs/*")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		s.jsonError(w, fmt.Sprintf("git fetch failed: %v: %s", err, output), http.StatusInternalServerError)
		return
	}

	req.Metadata.ReplicaOf = &storage.ReplicaSource{
		InstanceID:    req.InstanceID,
		InvitationKey: req.InvitationKey,
	}

	if err := s.storage.SetMetadata(req.Owner, req.Repo, req.Metadata); err != nil {
		s.jsonError(w, fmt.Sprintf("set metadata failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (s *Server) handleRegisterReplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Owner            string `json:"owner"`
		Repo             string `json:"repo"`
		ReplicaURL       string `json:"replica_url"`
		Token            string `json:"token"`
		OriginInstanceID string `json:"origin_instance_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Owner == "" || req.Repo == "" || req.Token == "" || req.OriginInstanceID == "" || req.ReplicaURL == "" {
		s.jsonError(w, "owner, repo, token, origin_instance_id, and replica_url required", http.StatusBadRequest)
		return
	}

	if !isValidName(req.Owner) || !isValidName(req.Repo) {
		s.jsonError(w, "invalid owner or repo name", http.StatusBadRequest)
		return
	}

	if !s.storage.RepoExists(req.Owner, req.Repo) {
		s.jsonError(w, "repository not found", http.StatusNotFound)
		return
	}

	meta, err := s.storage.GetMetadata(req.Owner, req.Repo)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get metadata failed: %v", err), http.StatusInternalServerError)
		return
	}

	if meta.ReplicaOf != nil {
		s.jsonError(w, "cannot add replica: this is a replica itself", http.StatusForbidden)
		return
	}

	replicationUser := fmt.Sprintf("replication-%s-%s-%s", req.Owner, req.Repo, req.OriginInstanceID)

	if err := s.authStore.CreateUserWithToken(replicationUser, "replication", req.Token); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			s.jsonError(w, fmt.Sprintf("create user failed: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func isValidName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return !strings.HasPrefix(name, ".") && !strings.HasSuffix(name, ".")
}
