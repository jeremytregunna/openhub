package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/jeremytregunna/openhub/internal/config"
	"github.com/jeremytregunna/openhub/internal/instance"
	"github.com/jeremytregunna/openhub/internal/storage"
)

func runAdmin(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: openhub admin <command> [args...]")
		fmt.Println("commands:")
		fmt.Println("  create-repo <owner/name>")
		fmt.Println("  delete-repo <owner/name>")
		fmt.Println("  list-repos [owner]")
		fmt.Println("  get-metadata <owner/name>")
		fmt.Println("  set-description <owner/name> <description>")
		fmt.Println("  add-replica <owner/name> <url>")
		fmt.Println("  remove-replica <owner/name> <instance-id>")
		fmt.Println("  list-replicas <owner/name>")
		fmt.Println("  recovery-bundle <owner/name>")
		os.Exit(1)
	}

	cmd := args[0]

	switch cmd {
	case "create-repo":
		if len(args) < 2 {
			fmt.Println("usage: openhub admin create-repo <owner/name>")
			os.Exit(1)
		}
		adminCreateRepo(args[1])
	case "delete-repo":
		if len(args) < 2 {
			fmt.Println("usage: openhub admin delete-repo <owner/name>")
			os.Exit(1)
		}
		adminDeleteRepo(args[1])
	case "list-repos":
		owner := ""
		if len(args) >= 2 {
			owner = args[1]
		}
		adminListRepos(owner)
	case "get-metadata":
		if len(args) < 2 {
			fmt.Println("usage: openhub admin get-metadata <owner/name>")
			os.Exit(1)
		}
		adminGetMetadata(args[1])
	case "set-description":
		if len(args) < 3 {
			fmt.Println("usage: openhub admin set-description <owner/name> <description>")
			os.Exit(1)
		}
		adminSetDescription(args[1], args[2])
	case "add-replica":
		if len(args) < 3 {
			fmt.Println("usage: openhub admin add-replica <owner/name> <url>")
			os.Exit(1)
		}
		adminAddReplica(args[1], args[2])
	case "remove-replica":
		if len(args) < 3 {
			fmt.Println("usage: openhub admin remove-replica <owner/name> <instance-id>")
			os.Exit(1)
		}
		adminRemoveReplica(args[1], args[2])
	case "list-replicas":
		if len(args) < 2 {
			fmt.Println("usage: openhub admin list-replicas <owner/name>")
			os.Exit(1)
		}
		adminListReplicas(args[1])
	case "recovery-bundle":
		if len(args) < 2 {
			fmt.Println("usage: openhub admin recovery-bundle <owner/name>")
			os.Exit(1)
		}
		adminRecoveryBundle(args[1])
	default:
		fmt.Printf("unknown admin command: %s\n", cmd)
		os.Exit(1)
	}
}

func adminCreateRepo(path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]

	apiURL := os.Getenv("OPENHUB_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000"
	}

	reqBody := map[string]string{
		"owner": owner,
		"name":  name,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("json error: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.Post(apiURL+"/api/repos/create", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Repository created: %s/%s\n", owner, name)
		if cloneURL, ok := result["clone_url"].(string); ok {
			fmt.Printf("Clone URL: %s\n", cloneURL)
		}
	} else {
		if errMsg, ok := result["error"].(string); ok {
			fmt.Printf("error: %s\n", errMsg)
		} else {
			fmt.Println("error: unknown failure")
		}
		os.Exit(1)
	}
}

func adminDeleteRepo(path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]

	apiURL := os.Getenv("OPENHUB_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000"
	}

	reqBody := map[string]string{
		"owner": owner,
		"name":  name,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("json error: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.Post(apiURL+"/api/repos/delete", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Repository deleted: %s/%s\n", owner, name)
	} else {
		if errMsg, ok := result["error"].(string); ok {
			fmt.Printf("error: %s\n", errMsg)
		} else {
			fmt.Println("error: unknown failure")
		}
		os.Exit(1)
	}
}

func adminListRepos(owner string) {
	apiURL := os.Getenv("OPENHUB_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000"
	}

	url := apiURL + "/api/repos/list"
	if owner != "" {
		url += "?owner=" + owner
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	if success, ok := result["success"].(bool); ok && success {
		if repos, ok := result["repos"].([]interface{}); ok {
			if len(repos) == 0 {
				fmt.Println("No repositories found")
				return
			}
			for _, r := range repos {
				if repo, ok := r.(map[string]interface{}); ok {
					owner := repo["owner"].(string)
					name := repo["name"].(string)
					fmt.Printf("%s/%s\n", owner, name)
				}
			}
		}
	} else {
		if errMsg, ok := result["error"].(string); ok {
			fmt.Printf("error: %s\n", errMsg)
		} else {
			fmt.Println("error: unknown failure")
		}
		os.Exit(1)
	}
}

func adminGetMetadata(path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]

	apiURL := os.Getenv("OPENHUB_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000"
	}

	url := fmt.Sprintf("%s/api/repos/metadata?owner=%s&name=%s", apiURL, owner, name)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	if success, ok := result["success"].(bool); ok && success {
		if metadata, ok := result["metadata"].(map[string]interface{}); ok {
			fmt.Printf("Repository: %s/%s\n", owner, name)
			if desc, ok := metadata["description"].(string); ok {
				fmt.Printf("Description: %s\n", desc)
			}
			if priv, ok := metadata["private"].(bool); ok {
				fmt.Printf("Private: %v\n", priv)
			}
			if branch, ok := metadata["default_branch"].(string); ok {
				fmt.Printf("Default Branch: %s\n", branch)
			}
			if created, ok := metadata["created_at"].(string); ok {
				fmt.Printf("Created: %s\n", created)
			}
		}
	} else {
		if errMsg, ok := result["error"].(string); ok {
			fmt.Printf("error: %s\n", errMsg)
		} else {
			fmt.Println("error: unknown failure")
		}
		os.Exit(1)
	}
}

func adminSetDescription(path, description string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]

	apiURL := os.Getenv("OPENHUB_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:3000"
	}

	getURL := fmt.Sprintf("%s/api/repos/metadata?owner=%s&name=%s", apiURL, owner, name)
	resp, err := http.Get(getURL)
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		os.Exit(1)
	}

	var getResult map[string]interface{}
	if err := json.Unmarshal(body, &getResult); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	var metadata map[string]interface{}
	if meta, ok := getResult["metadata"].(map[string]interface{}); ok {
		metadata = meta
	} else {
		metadata = make(map[string]interface{})
	}

	metadata["description"] = description

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		fmt.Printf("json error: %v\n", err)
		os.Exit(1)
	}

	setURL := fmt.Sprintf("%s/api/repos/metadata?owner=%s&name=%s", apiURL, owner, name)
	resp, err = http.Post(setURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("request error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("read error: %v\n", err)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	if success, ok := result["success"].(bool); ok && success {
		fmt.Printf("Description updated for %s/%s\n", owner, name)
	} else {
		if errMsg, ok := result["error"].(string); ok {
			fmt.Printf("error: %s\n", errMsg)
		} else {
			fmt.Println("error: unknown failure")
		}
		os.Exit(1)
	}
}

func getStorage() *storage.Storage {
	cfg := config.Default()
	if storagePath := os.Getenv("OPENHUB_STORAGE"); storagePath != "" {
		cfg.StoragePath = storagePath
	}

	store, err := storage.New(cfg.StoragePath)
	if err != nil {
		fmt.Printf("storage init: %v\n", err)
		os.Exit(1)
	}
	return store
}

func adminAddReplica(path, url string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]
	cfg := config.Default()
	if storagePath := os.Getenv("OPENHUB_STORAGE"); storagePath != "" {
		cfg.StoragePath = storagePath
	}

	store := getStorage()
	inst, err := instance.LoadOrCreate(cfg.StoragePath)
	if err != nil {
		fmt.Printf("error loading instance: %v\n", err)
		os.Exit(1)
	}

	meta, err := store.GetMetadata(owner, name)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	if meta.ReplicaOf != nil {
		fmt.Println("error: cannot add replica to a replica repository")
		os.Exit(1)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		fmt.Printf("error generating token: %v\n", err)
		os.Exit(1)
	}
	token := hex.EncodeToString(tokenBytes)

	invitationKeyBytes := make([]byte, 32)
	if _, err := rand.Read(invitationKeyBytes); err != nil {
		fmt.Printf("error generating invitation key: %v\n", err)
		os.Exit(1)
	}
	invitationKey := hex.EncodeToString(invitationKeyBytes)

	fmt.Println("Registering with replica...")

	reqBody := map[string]string{
		"owner":              owner,
		"repo":               name,
		"replica_url":        url,
		"token":              token,
		"origin_instance_id": inst.ID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("json error: %v\n", err)
		os.Exit(1)
	}

	registerURL := url + "/api/repos/register-replication"
	resp, err := http.Post(registerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("error contacting replica: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("error reading response: %v\n", err)
		os.Exit(1)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("json decode error: %v\n", err)
		os.Exit(1)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		if errMsg, ok := result["error"].(string); ok {
			fmt.Printf("replica registration failed: %s\n", errMsg)
		} else {
			fmt.Println("replica registration failed: unknown error")
		}
		os.Exit(1)
	}

	replica := storage.Replica{
		InstanceID:    inst.ID,
		URL:           url,
		Token:         token,
		InvitationKey: invitationKey,
		Enabled:       true,
	}

	meta.Replicas = append(meta.Replicas, replica)

	if err := store.SetMetadata(owner, name, meta); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Replica configured successfully\n")
	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Invitation Key: %s\n", invitationKey)
	fmt.Println("\nShare this invitation key with the replica administrator.")
	fmt.Println("They need it to accept replication from this origin.")
	fmt.Printf("Replica will receive updates on push\n")
}

func adminRemoveReplica(path, instanceID string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]
	store := getStorage()

	meta, err := store.GetMetadata(owner, name)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	var newReplicas []storage.Replica
	found := false
	for _, r := range meta.Replicas {
		if r.InstanceID == instanceID {
			found = true
			continue
		}
		newReplicas = append(newReplicas, r)
	}

	if !found {
		fmt.Printf("replica with instance ID %s not found\n", instanceID)
		os.Exit(1)
	}

	meta.Replicas = newReplicas

	if err := store.SetMetadata(owner, name, meta); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Replica removed from %s/%s\n", owner, name)
}

func adminListReplicas(path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]
	store := getStorage()

	meta, err := store.GetMetadata(owner, name)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	if len(meta.Replicas) == 0 {
		fmt.Println("No replicas configured")
		return
	}

	fmt.Printf("Replicas for %s/%s:\n", owner, name)
	for i, r := range meta.Replicas {
		status := "enabled"
		if !r.Enabled {
			status = "disabled"
		}
		fmt.Printf("%d. URL: %s\n", i+1, r.URL)
		fmt.Printf("   Instance ID: %s\n", r.InstanceID)
		fmt.Printf("   Invitation Key: %s\n", r.InvitationKey)
		fmt.Printf("   Status: %s\n", status)
		if !r.LastSynced.IsZero() {
			fmt.Printf("   Last Synced: %s\n", r.LastSynced.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}
}

func adminRecoveryBundle(path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		fmt.Println("invalid repo path, must be owner/name")
		os.Exit(1)
	}

	owner, name := parts[0], parts[1]
	store := getStorage()

	meta, err := store.GetMetadata(owner, name)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}

	bundle := map[string]interface{}{
		"repo":  fmt.Sprintf("%s/%s", owner, name),
		"replicas": meta.Replicas,
	}

	jsonData, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Printf("json error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}
