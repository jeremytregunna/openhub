package main

import (
	"fmt"
	"os"

	"github.com/jeremytregunna/openhub/internal/auth"
	"github.com/jeremytregunna/openhub/internal/config"
)

func runUser(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: openhub user <command> [args...]")
		fmt.Println("commands:")
		fmt.Println("  create <username>")
		fmt.Println("  add-key <username> <key-name> <ssh-public-key>")
		fmt.Println("  generate-token <username> <token-name>")
		os.Exit(1)
	}

	cmd := args[0]

	cfg := config.Default()
	if storagePath := os.Getenv("OPENHUB_STORAGE"); storagePath != "" {
		cfg.StoragePath = storagePath
	}

	authStore, err := auth.NewAuthStore(cfg.StoragePath)
	if err != nil {
		fmt.Printf("auth store init: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "create":
		if len(args) < 2 {
			fmt.Println("usage: openhub user create <username>")
			os.Exit(1)
		}
		userCreate(authStore, args[1])
	case "add-key":
		if len(args) < 4 {
			fmt.Println("usage: openhub user add-key <username> <key-name> <ssh-public-key>")
			os.Exit(1)
		}
		userAddKey(authStore, args[1], args[2], args[3])
	case "generate-token":
		if len(args) < 3 {
			fmt.Println("usage: openhub user generate-token <username> <token-name>")
			os.Exit(1)
		}
		userGenerateToken(authStore, args[1], args[2])
	default:
		fmt.Printf("unknown user command: %s\n", cmd)
		os.Exit(1)
	}
}

func userCreate(authStore *auth.AuthStore, username string) {
	if err := authStore.CreateUser(username); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("User created: %s\n", username)
}

func userAddKey(authStore *auth.AuthStore, username, keyName, key string) {
	if err := authStore.AddSSHKey(username, keyName, key); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("SSH key added for user %s\n", username)
}

func userGenerateToken(authStore *auth.AuthStore, username, tokenName string) {
	token, err := authStore.GenerateAPIToken(username, tokenName)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("API token generated for user %s:\n", username)
	fmt.Printf("%s\n", token)
	fmt.Println("")
	fmt.Println("Use this token in API requests:")
	fmt.Println("  curl -H \"Authorization: Bearer <token>\" ...")
}
