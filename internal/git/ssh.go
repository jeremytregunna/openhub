package git

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"

	"github.com/jeremytregunna/openhub/internal/storage"
	"golang.org/x/crypto/ssh"
)

type SSHServer struct {
	config     *ssh.ServerConfig
	storage    RepoStorage
	authStore  AuthStore
	replQueue  ReplicationQueue
	port       int
	userConns  map[*ssh.ServerConn]string
}

type RepoStorage interface {
	RepoPath(owner, name string) string
	RepoExists(owner, name string) bool
	GetMetadata(owner, name string) (storage.Metadata, error)
}

type AuthStore interface {
	ValidateSSHKey(key string) (string, error)
}

type ReplicationQueue interface {
	Queue(owner, repo string)
}

func NewSSHServer(port int, storage RepoStorage, authStore AuthStore, hostKey ssh.Signer, replQueue ReplicationQueue) *SSHServer {
	s := &SSHServer{
		storage:   storage,
		authStore: authStore,
		replQueue: replQueue,
		port:      port,
		userConns: make(map[*ssh.ServerConn]string),
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: s.publicKeyCallback,
	}
	config.AddHostKey(hostKey)
	s.config = config

	return s
}

func (s *SSHServer) publicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	keyStr := string(ssh.MarshalAuthorizedKey(key))
	username, err := s.authStore.ValidateSSHKey(strings.TrimSpace(keyStr))
	if err != nil {
		return nil, fmt.Errorf("invalid key")
	}

	return &ssh.Permissions{
		Extensions: map[string]string{
			"user": username,
		},
	}, nil
}

func (s *SSHServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Printf("SSH server listening on port %d", s.port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *SSHServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		log.Printf("handshake error: %v", err)
		return
	}
	defer sshConn.Close()

	username := ""
	if sshConn.Permissions != nil && sshConn.Permissions.Extensions != nil {
		username = sshConn.Permissions.Extensions["user"]
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("accept channel error: %v", err)
			continue
		}

		go s.handleSession(channel, requests, username)
	}
}

func (s *SSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request, username string) {
	defer channel.Close()

	for req := range requests {
		switch req.Type {
		case "exec":
			command := string(req.Payload[4:])
			req.Reply(true, nil)
			s.handleGitCommand(channel, command, username)
			return
		case "shell":
			req.Reply(false, nil)
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

func (s *SSHServer) handleGitCommand(channel ssh.Channel, command, username string) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		fmt.Fprintf(channel.Stderr(), "invalid command\n")
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
		return
	}

	gitCmd := parts[0]
	repoPath := strings.Trim(parts[1], "'\"")

	if !strings.HasPrefix(gitCmd, "git-") {
		fmt.Fprintf(channel.Stderr(), "only git commands allowed\n")
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
		return
	}

	owner, repo := parseRepoPath(repoPath)
	if owner == "" || repo == "" {
		fmt.Fprintf(channel.Stderr(), "invalid repo path\n")
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
		return
	}

	if !s.storage.RepoExists(owner, repo) {
		fmt.Fprintf(channel.Stderr(), "repository not found\n")
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
		return
	}

	meta, err := s.storage.GetMetadata(owner, repo)
	if err != nil {
		fmt.Fprintf(channel.Stderr(), "error getting metadata\n")
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
		return
	}

	needsWrite := gitCmd == "git-receive-pack"

	if needsWrite {
		if meta.ReplicaOf != nil {
			fmt.Fprintf(channel.Stderr(), "permission denied: repository is a read-only replica\n")
			channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
			return
		}
		if username != owner {
			fmt.Fprintf(channel.Stderr(), "permission denied: only owner can push\n")
			channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
			return
		}
	} else {
		if meta.Private && username != owner {
			fmt.Fprintf(channel.Stderr(), "permission denied: private repository\n")
			channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
			return
		}
	}

	fullPath := s.storage.RepoPath(owner, repo)
	cmd := exec.Command(gitCmd, fullPath)
	cmd.Stdin = channel
	cmd.Stdout = channel
	cmd.Stderr = channel.Stderr()

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(channel.Stderr(), "command error: %v\n", err)
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
		return
	}

	if needsWrite && s.replQueue != nil {
		s.replQueue.Queue(owner, repo)
	}

	channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
}

func parseRepoPath(path string) (owner, repo string) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
