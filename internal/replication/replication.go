package replication

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/jeremytregunna/openhub/internal/storage"
)

type Job struct {
	Owner string
	Repo  string
}

type Manager struct {
	store      *storage.Storage
	instanceID string
	queue      chan Job
	wg         sync.WaitGroup
}

func NewManager(store *storage.Storage, instanceID string) *Manager {
	return &Manager{
		store:      store,
		instanceID: instanceID,
		queue:      make(chan Job, 100),
	}
}

func (m *Manager) Start(workers int) {
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}
}

func (m *Manager) Stop() {
	close(m.queue)
	m.wg.Wait()
}

func (m *Manager) Queue(owner, repo string) {
	select {
	case m.queue <- Job{Owner: owner, Repo: repo}:
	default:
		log.Printf("replication queue full, dropping job for %s/%s", owner, repo)
	}
}

func (m *Manager) worker() {
	defer m.wg.Done()

	for job := range m.queue {
		if err := m.replicate(job.Owner, job.Repo); err != nil {
			log.Printf("replication failed for %s/%s: %v", job.Owner, job.Repo, err)
		}
	}
}

func (m *Manager) replicate(owner, repo string) error {
	meta, err := m.store.GetMetadata(owner, repo)
	if err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}

	if len(meta.Replicas) == 0 {
		return nil
	}

	bundle, err := m.createBundle(owner, repo)
	if err != nil {
		return fmt.Errorf("create bundle: %w", err)
	}

	for i, replica := range meta.Replicas {
		if !replica.Enabled {
			continue
		}

		log.Printf("pushing to replica %s", replica.URL)
		if err := m.pushToReplica(owner, repo, replica, bundle); err != nil {
			log.Printf("push to replica %s failed: %v", replica.URL, err)
			continue
		}

		log.Printf("successfully replicated to %s", replica.URL)
		meta.Replicas[i].LastSynced = time.Now()
	}

	if err := m.store.SetMetadata(owner, repo, meta); err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}

	return nil
}

func (m *Manager) createBundle(owner, repo string) ([]byte, error) {
	repoPath := m.store.RepoPath(owner, repo)

	cmd := exec.Command("git", "bundle", "create", "-", "--all")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git bundle: %w", err)
	}

	return output, nil
}

func (m *Manager) pushToReplica(owner, repo string, replica storage.Replica, bundle []byte) error {
	url := fmt.Sprintf("%s/api/repos/replicate", replica.URL)

	meta, err := m.store.GetMetadata(owner, repo)
	if err != nil {
		return fmt.Errorf("get metadata: %w", err)
	}

	metaCopy := meta
	metaCopy.Replicas = nil

	payload := map[string]interface{}{
		"owner":          owner,
		"repo":           repo,
		"instance_id":    m.instanceID,
		"invitation_key": replica.InvitationKey,
		"bundle":         base64.StdEncoding.EncodeToString(bundle),
		"metadata":       metaCopy,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+replica.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("replica returned %d: %s", resp.StatusCode, body)
	}

	return nil
}

func (m *Manager) SyncAll() {
	repos, err := m.store.ListRepos()
	if err != nil {
		log.Printf("sync all: list repos failed: %v", err)
		return
	}

	for _, repo := range repos {
		m.Queue(repo.Owner, repo.Name)
	}
}

func (m *Manager) StartPeriodicSync(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Println("starting periodic replication sync")
			m.SyncAll()
		}
	}()
}
