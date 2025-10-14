package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Storage struct {
	basePath string
}

func New(basePath string) (*Storage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &Storage{basePath: basePath}, nil
}

func (s *Storage) RepoPath(owner, name string) string {
	return filepath.Join(s.basePath, owner, name+".git")
}

func (s *Storage) RepoExists(owner, name string) bool {
	path := s.RepoPath(owner, name)
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (s *Storage) CreateRepo(owner, name string) error {
	path := s.RepoPath(owner, name)

	if s.RepoExists(owner, name) {
		return fmt.Errorf("repo already exists: %s/%s", owner, name)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create owner dir: %w", err)
	}

	cmd := exec.Command("git", "init", "--bare", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	meta := Metadata{
		Description:   "",
		Private:       false,
		DefaultBranch: "main",
		CreatedAt:     time.Now(),
	}

	if err := s.SetMetadata(owner, name, meta); err != nil {
		return fmt.Errorf("set metadata: %w", err)
	}

	return nil
}

func (s *Storage) DeleteRepo(owner, name string) error {
	path := s.RepoPath(owner, name)

	if !s.RepoExists(owner, name) {
		return fmt.Errorf("repo does not exist: %s/%s", owner, name)
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("delete repo: %w", err)
	}

	return nil
}

type Repo struct {
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type Replica struct {
	InstanceID    string    `json:"instance_id"`
	URL           string    `json:"url"`
	Token         string    `json:"token"`
	InvitationKey string    `json:"invitation_key"`
	Enabled       bool      `json:"enabled"`
	LastSynced    time.Time `json:"last_synced,omitempty"`
}

type ReplicaSource struct {
	InstanceID    string `json:"instance_id"`
	InvitationKey string `json:"invitation_key"`
}

type Metadata struct {
	Description   string         `json:"description"`
	Private       bool           `json:"private"`
	DefaultBranch string         `json:"default_branch"`
	CreatedAt     time.Time      `json:"created_at"`
	Replicas      []Replica      `json:"replicas,omitempty"`
	ReplicaOf     *ReplicaSource `json:"replica_of,omitempty"`
}

func (s *Storage) ListRepos() ([]Repo, error) {
	var repos []Repo

	owners, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("read storage dir: %w", err)
	}

	for _, owner := range owners {
		if !owner.IsDir() {
			continue
		}

		ownerPath := filepath.Join(s.basePath, owner.Name())
		entries, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".git") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".git")
			repos = append(repos, Repo{
				Owner: owner.Name(),
				Name:  name,
			})
		}
	}

	return repos, nil
}

func (s *Storage) ListReposByOwner(owner string) ([]Repo, error) {
	var repos []Repo

	ownerPath := filepath.Join(s.basePath, owner)
	entries, err := os.ReadDir(ownerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return repos, nil
		}
		return nil, fmt.Errorf("read owner dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".git") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".git")
		repos = append(repos, Repo{
			Owner: owner,
			Name:  name,
		})
	}

	return repos, nil
}

func (s *Storage) metadataPath(owner, name string) string {
	return filepath.Join(s.RepoPath(owner, name), "openhub.json")
}

func (s *Storage) GetMetadata(owner, name string) (Metadata, error) {
	var meta Metadata

	if !s.RepoExists(owner, name) {
		return meta, fmt.Errorf("repo does not exist: %s/%s", owner, name)
	}

	path := s.metadataPath(owner, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Metadata{
				Description:   "",
				Private:       false,
				DefaultBranch: "main",
				CreatedAt:     time.Time{},
			}, nil
		}
		return meta, fmt.Errorf("read metadata: %w", err)
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return meta, nil
}

func (s *Storage) SetMetadata(owner, name string, meta Metadata) error {
	if !s.RepoExists(owner, name) {
		return fmt.Errorf("repo does not exist: %s/%s", owner, name)
	}

	path := s.metadataPath(owner, name)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}
