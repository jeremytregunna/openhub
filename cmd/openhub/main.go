package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "server":
		runServer(os.Args[2:])
	case "admin":
		runAdmin(os.Args[2:])
	case "user":
		runUser(os.Args[2:])
	default:
		fmt.Printf("unknown command: %s\n", command)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("openhub - Federated Git Hosting")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  server            Start the git server")
	fmt.Println("  admin             Admin commands")
	fmt.Println("  user              User management")
	fmt.Println("")
	fmt.Println("Server flags:")
	fmt.Println("  --ssh-port        SSH server port (default: 2222)")
	fmt.Println("  --http-port       HTTP server port (default: 3000)")
	fmt.Println("")
	fmt.Println("Admin subcommands:")
	fmt.Println("  create-repo       Create a new repository")
	fmt.Println("  delete-repo       Delete a repository")
	fmt.Println("  list-repos        List repositories")
	fmt.Println("  get-metadata      Get repository metadata")
	fmt.Println("  set-description   Set repository description")
	fmt.Println("  add-replica       Add replica (auto-registers with remote)")
	fmt.Println("  remove-replica    Remove a replica")
	fmt.Println("  list-replicas     List configured replicas")
	fmt.Println("  recovery-bundle   Generate recovery bundle JSON")
	fmt.Println("")
	fmt.Println("User subcommands:")
	fmt.Println("  create            Create a new user")
	fmt.Println("  add-key           Add SSH key to user")
	fmt.Println("  generate-token    Generate API token for user")
}
