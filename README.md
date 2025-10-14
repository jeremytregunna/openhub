# OpenHub

Self-hosted Git server with SSH and HTTP access.

## Features

- Git hosting (SSH + HTTP)
- Public and private repositories
- Optional replication (see [docs/FEDERATION.md](docs/FEDERATION.md))
- User management, supports SSH keys and API tokens (HTTP)

## Quick Start

### Running with Docker

```bash
docker-compose up -d
docker exec openhub ./openhub user create alice
docker exec openhub ./openhub admin create-repo alice/myproject
```

Server runs on:
- SSH: port 2222
- HTTP: port 3000

### Running from Source

```bash
go build ./cmd/openhub
./openhub server

# Custom storage path
OPENHUB_STORAGE=/path/to/storage ./openhub server

# Custom ports
./openhub server --ssh-port 2222 --http-port 3000
```

## Usage

### Setup

```bash
# Create user
./openhub user create alice

# Create repository
./openhub admin create-repo alice/myproject
```

### Authentication

```bash
# Add SSH key
./openhub user add-key alice laptop "ssh-ed25519 AAAAC3... alice@laptop"

# Generate API token for HTTP
./openhub user generate-token alice mytoken
```

### Using Git

**SSH:**
```bash
git remote add origin ssh://alice@localhost:2222/alice/myproject.git
git push origin master
```

**HTTP:**
```bash
git remote add origin http://localhost:3000/alice/myproject.git
git push origin master
# Username: alice
# Password: <api-token>
```

### Repository Management

```bash
# List repositories
./openhub admin list-repos alice

# Get metadata
./openhub admin get-metadata alice/myproject

# Set description
./openhub admin set-description alice/myproject "My project"

# Delete repository
./openhub admin delete-repo alice/myproject
```

## More

- **Replication**: See [docs/FEDERATION.md](docs/FEDERATION.md) for backup/redundancy setup

## License

Copyright 2025 Jeremy Tregunna

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
