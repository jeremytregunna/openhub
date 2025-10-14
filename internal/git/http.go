package git

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"path"
	"strings"
)

type TokenValidator interface {
	ValidateAPIToken(token string) (string, error)
}

type HTTPServer struct {
	storage   RepoStorage
	validator TokenValidator
	mux       *http.ServeMux
}

func NewHTTPServer(storage RepoStorage, validator TokenValidator) *HTTPServer {
	s := &HTTPServer{
		storage:   storage,
		validator: validator,
		mux:       http.NewServeMux(),
	}

	s.mux.HandleFunc("/", s.handleRequest)

	return s
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *HTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/info/refs") {
		s.handleInfoRefs(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/git-upload-pack") {
		s.handleGitCommand(w, r, "git-upload-pack")
		return
	}

	if strings.HasSuffix(r.URL.Path, "/git-receive-pack") {
		s.handleGitCommand(w, r, "git-receive-pack")
		return
	}

	http.NotFound(w, r)
}

func (s *HTTPServer) getAuthenticatedUser(r *http.Request) string {
	username, password, ok := r.BasicAuth()
	if !ok || username == "" || password == "" {
		return ""
	}

	validatedUser, err := s.validator.ValidateAPIToken(password)
	if err != nil {
		return ""
	}

	if validatedUser != username {
		return ""
	}

	return username
}

func (s *HTTPServer) handleInfoRefs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service := r.URL.Query().Get("service")
	if service != "git-upload-pack" && service != "git-receive-pack" {
		http.Error(w, "invalid service", http.StatusBadRequest)
		return
	}

	owner, repo := s.parseRepoPath(r.URL.Path)
	if owner == "" || repo == "" {
		http.Error(w, "invalid repo path", http.StatusBadRequest)
		return
	}

	if !s.storage.RepoExists(owner, repo) {
		http.NotFound(w, r)
		return
	}

	meta, err := s.storage.GetMetadata(owner, repo)
	if err != nil {
		http.Error(w, "error getting metadata", http.StatusInternalServerError)
		return
	}

	username := s.getAuthenticatedUser(r)

	needsWrite := service == "git-receive-pack"
	if needsWrite {
		if meta.ReplicaOf != nil {
			http.Error(w, "cannot push: repository is a read-only replica", http.StatusForbidden)
			return
		}
		if username != owner {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Git\"")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		if meta.Private && username != owner {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Git\"")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	repoPath := s.storage.RepoPath(owner, repo)

	cmd := exec.Command(service, "--stateless-rpc", "--advertise-refs", repoPath)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("git command error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")

	servicePacket := fmt.Sprintf("# service=%s\n", service)
	packetLen := len(servicePacket) + 4
	fmt.Fprintf(w, "%04x%s", packetLen, servicePacket)
	fmt.Fprint(w, "0000")
	w.Write(out)
}

func (s *HTTPServer) handleGitCommand(w http.ResponseWriter, r *http.Request, service string) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	owner, repo := s.parseRepoPath(r.URL.Path)
	if owner == "" || repo == "" {
		http.Error(w, "invalid repo path", http.StatusBadRequest)
		return
	}

	if !s.storage.RepoExists(owner, repo) {
		http.NotFound(w, r)
		return
	}

	meta, err := s.storage.GetMetadata(owner, repo)
	if err != nil {
		http.Error(w, "error getting metadata", http.StatusInternalServerError)
		return
	}

	username := s.getAuthenticatedUser(r)

	needsWrite := service == "git-receive-pack"
	if needsWrite {
		if meta.ReplicaOf != nil {
			http.Error(w, "cannot push: repository is a read-only replica", http.StatusForbidden)
			return
		}
		if username != owner {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Git\"")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		if meta.Private && username != owner {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Git\"")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	repoPath := s.storage.RepoPath(owner, repo)

	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "invalid gzip", http.StatusBadRequest)
			return
		}
		defer gz.Close()
		body = gz
	}

	cmd := exec.Command(service, "--stateless-rpc", repoPath)
	cmd.Stdin = body

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := cmd.Start(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-result", service))
	w.Header().Set("Cache-Control", "no-cache")

	io.Copy(w, stdout)
	cmd.Wait()
}

func (s *HTTPServer) parseRepoPath(urlPath string) (owner, repo string) {
	urlPath = path.Clean(urlPath)
	urlPath = strings.TrimPrefix(urlPath, "/")

	if strings.HasSuffix(urlPath, "/info/refs") {
		urlPath = strings.TrimSuffix(urlPath, "/info/refs")
	}
	if strings.HasSuffix(urlPath, "/git-upload-pack") {
		urlPath = strings.TrimSuffix(urlPath, "/git-upload-pack")
	}
	if strings.HasSuffix(urlPath, "/git-receive-pack") {
		urlPath = strings.TrimSuffix(urlPath, "/git-receive-pack")
	}

	urlPath = strings.TrimSuffix(urlPath, ".git")
	parts := strings.Split(urlPath, "/")

	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
