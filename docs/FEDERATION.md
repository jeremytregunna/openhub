# Federation & Replication

OpenHub supports secure async replication for backup and redundancy. Replication requires mutual consent between origin and replica servers.

## Architecture

- **Origin servers**: Accept pushes, manage replicas
- **Replica servers**: Read-only copies, reject pushes
- **Async replication**: Background workers push git bundles
- **Periodic sync**: Every 5 minutes all repos sync to replicas
- **Invitation-based**: Unique keys prevent unauthorized replication

## Setting Up Replication

### On Origin Server

Add a replica endpoint:

```bash
./openhub admin add-replica alice/myproject http://replica.example.com:3000
```

Output:
```
Registering with replica...
✓ Replica configured successfully
URL: http://replica.example.com:3000
Invitation Key: a1b2c3d4e5f6789abcdef...

Share this invitation key with the replica administrator.
They need it to accept replication from this origin.
Replica will receive updates on push
```

### Share Invitation Key

Securely share the invitation key with replica administrator via:
- Signal/encrypted messaging
- PGP-encrypted email
- In-person/phone call

### Trigger Replication

Push to origin triggers replication:

```bash
git push openhub master
```

Check logs:
```bash
# You should see:
# "pushing to replica http://replica.example.com:3000"
# "successfully replicated to http://replica.example.com:3000"
```

## Managing Replicas

```bash
# List replicas for a repository
./openhub admin list-replicas alice/myproject

# Remove a replica
./openhub admin remove-replica alice/myproject <instance-id>

# Generate recovery bundle
./openhub admin recovery-bundle alice/myproject > recovery.json
```

## Security Model

### What's Protected

- **Path traversal**: Validated owner/repo names block `../` attacks
- **Mutual consent**: Replicas validate invitation keys
- **Read-only replicas**: Push attempts rejected
- **Private repos**: Privacy preserved on replicas
- **Scoped credentials**: Unique token per repo/replica pair

### Authentication Flow

1. Origin generates invitation key + replication token
2. Origin calls replica's register endpoint
3. Replica creates scoped user: `replication-{owner}-{repo}-{instanceID}`
4. On push, origin sends bundle + metadata + invitation key
5. Replica validates invitation key, applies bundle
6. Replica stores `ReplicaOf` metadata, rejects future pushes

### Replica Isolation

Replicas cannot:
- Accept pushes (read-only)
- Create their own replicas (cascade prevented)
- Access private repos without proper invitation key
- Overwrite origin repos (instance IDs prevent loops)

## Multi-Instance Testing

Run two instances locally to test replication:

```bash
# Terminal 1: Start first instance (origin)
OPENHUB_STORAGE=/tmp/openhub1 ./openhub server --ssh-port 2222 --http-port 3000

# Terminal 2: Start second instance (replica)
OPENHUB_STORAGE=/tmp/openhub2 ./openhub server --ssh-port 2223 --http-port 3001

# Terminal 3: Set up replication
OPENHUB_STORAGE=/tmp/openhub1 ./openhub user create alice
OPENHUB_STORAGE=/tmp/openhub1 ./openhub admin create-repo alice/myproject
OPENHUB_STORAGE=/tmp/openhub1 ./openhub admin add-replica \
  alice/myproject \
  http://localhost:3001
```

## Recovery

Generate recovery bundles containing replica configuration:

```bash
./openhub admin recovery-bundle alice/myproject > recovery.json
```

Store safely. Contains:
- Repository name
- Replica URLs
- Replication tokens
- Invitation keys

Use to restore replica configuration after data loss.
