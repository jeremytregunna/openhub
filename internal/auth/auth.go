package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type User struct {
	Username  string    `json:"username"`
	SSHKeys   []SSHKey  `json:"ssh_keys"`
	APITokens []APIToken `json:"api_tokens"`
	CreatedAt time.Time `json:"created_at"`
}

type SSHKey struct {
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	AddedAt   time.Time `json:"added_at"`
}

type APIToken struct {
	Name      string    `json:"name"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthStore struct {
	basePath string
}

func NewAuthStore(basePath string) (*AuthStore, error) {
	userDir := filepath.Join(basePath, "users")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return nil, fmt.Errorf("create users dir: %w", err)
	}
	return &AuthStore{basePath: basePath}, nil
}

func (a *AuthStore) userPath(username string) string {
	return filepath.Join(a.basePath, "users", username+".json")
}

func (a *AuthStore) GetUser(username string) (*User, error) {
	path := a.userPath(username)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("read user: %w", err)
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	return &user, nil
}

func (a *AuthStore) CreateUser(username string) error {
	if _, err := a.GetUser(username); err == nil {
		return fmt.Errorf("user already exists: %s", username)
	}

	user := User{
		Username:  username,
		SSHKeys:   []SSHKey{},
		APITokens: []APIToken{},
		CreatedAt: time.Now(),
	}

	return a.saveUser(&user)
}

func (a *AuthStore) CreateUserWithToken(username, tokenName, token string) error {
	if _, err := a.GetUser(username); err == nil {
		return fmt.Errorf("user already exists: %s", username)
	}

	user := User{
		Username: username,
		SSHKeys:  []SSHKey{},
		APITokens: []APIToken{
			{
				Name:      tokenName,
				Token:     token,
				CreatedAt: time.Now(),
			},
		},
		CreatedAt: time.Now(),
	}

	return a.saveUser(&user)
}

func (a *AuthStore) saveUser(user *User) error {
	path := a.userPath(user.Username)
	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write user: %w", err)
	}

	return nil
}

func (a *AuthStore) AddSSHKey(username, name, key string) error {
	user, err := a.GetUser(username)
	if err != nil {
		return err
	}

	user.SSHKeys = append(user.SSHKeys, SSHKey{
		Name:    name,
		Key:     key,
		AddedAt: time.Now(),
	})

	return a.saveUser(user)
}

func (a *AuthStore) GenerateAPIToken(username, name string) (string, error) {
	user, err := a.GetUser(username)
	if err != nil {
		return "", err
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	user.APITokens = append(user.APITokens, APIToken{
		Name:      name,
		Token:     token,
		CreatedAt: time.Now(),
	})

	if err := a.saveUser(user); err != nil {
		return "", err
	}

	return token, nil
}

func (a *AuthStore) ValidateAPIToken(token string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(a.basePath, "users"))
	if err != nil {
		return "", fmt.Errorf("read users dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		username := entry.Name()[:len(entry.Name())-5]
		user, err := a.GetUser(username)
		if err != nil {
			continue
		}

		for _, t := range user.APITokens {
			if t.Token == token {
				return user.Username, nil
			}
		}
	}

	return "", fmt.Errorf("invalid token")
}

func (a *AuthStore) ValidateSSHKey(key string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(a.basePath, "users"))
	if err != nil {
		return "", fmt.Errorf("read users dir: %w", err)
	}

	normalizedKey := normalizeSSHKey(key)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		username := entry.Name()[:len(entry.Name())-5]
		user, err := a.GetUser(username)
		if err != nil {
			continue
		}

		for _, k := range user.SSHKeys {
			if normalizeSSHKey(k.Key) == normalizedKey {
				return user.Username, nil
			}
		}
	}

	return "", fmt.Errorf("invalid SSH key")
}

func normalizeSSHKey(key string) string {
	parts := strings.Fields(key)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return strings.TrimSpace(key)
}

type Permission int

const (
	PermissionNone Permission = iota
	PermissionRead
	PermissionWrite
)

type PermissionChecker interface {
	GetMetadata(owner, name string) (Metadata, error)
}

type Metadata struct {
	Private bool
}

func CheckPermission(username, owner, repo string, checker PermissionChecker) Permission {
	if username == owner {
		return PermissionWrite
	}

	meta, err := checker.GetMetadata(owner, repo)
	if err != nil {
		return PermissionNone
	}

	if meta.Private {
		return PermissionNone
	}

	return PermissionRead
}
