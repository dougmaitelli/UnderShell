# SSH Realms

SSH Realms is a multiplayer RPG played entirely through a standard SSH client.
Your SSH public key is your character identity: a new key creates a new
character, while a known key returns directly to the shared world.

## Requirements

- Go 1.26 or newer
- An OpenSSH-compatible client

Docker is optional and is only required for the container deployment.

## Quick start

```sh
make run
```

The local server listens on port 2222 by default. In another terminal, create a
player key and connect:

```sh
ssh-keygen -t ed25519 -f /tmp/sshrpg-player -N ''
ssh -p 2222 -i /tmp/sshrpg-player -o IdentitiesOnly=yes localhost
```

Use a different SSH key to create another character.

## Controls

| Input | Action |
|---|---|
| WASD or arrow keys | Move |
| Enter | Submit the character name |
| Ctrl+C | Disconnect |

## Configuration

Configuration is supplied through environment variables. Invalid or non-positive
world dimensions fall back to their defaults.

| Variable | Default | Purpose |
|---|---|---|
| `SSH_LISTEN_ADDR` | `:2222` | Address and port used by the SSH server |
| `SSH_HOST_KEY_PATH` | `./data/ssh_host_ed25519` | Persistent Ed25519 server host-key path |
| `DATABASE_PATH` | `./data/game.db` | Persistent SQLite database path |
| `WORLD_WIDTH` | `120` | World width in tiles |
| `WORLD_HEIGHT` | `60` | World height in tiles |

Example:

```sh
SSH_LISTEN_ADDR=:2022 \
DATABASE_PATH=./data/development.db \
WORLD_WIDTH=160 \
WORLD_HEIGHT=80 \
make run
```

## Persistent data

Runtime state is written to `./data` with the default configuration:

- `game.db` stores characters and their positions.
- `ssh_host_ed25519` identifies the SSH server to clients.

Back up both files. If the host key changes, returning SSH clients will receive a
host-identity warning.

## Docker and port 22

The included Compose configuration publishes host port 22 to the unprivileged
game process on container port 2222:

```sh
docker compose up --build -d
ssh -i ~/.ssh/id_ed25519 game.example.com
```

Port 22 must not already be occupied. Keep administrative SSH on a different
address or port before starting the game, or use a dedicated host.

For a direct non-container deployment, set `SSH_LISTEN_ADDR=:22` and grant only
the bind capability to the binary:

```sh
make build
sudo setcap 'cap_net_bind_service=+ep' ./bin/sshrpg
SSH_LISTEN_ADDR=:22 ./bin/sshrpg
```

## Player identity

- Only clients that prove possession of a public key are admitted.
- Password authentication and SSH port forwarding are disabled.
- The SHA-256 public-key fingerprint identifies a character.
- User-provided names are length checked and reject terminal control characters.
- A newer login with the same character key disconnects the older session.
- The server never receives or stores client private keys.

Combat, collision, chat, inventory, and account/key recovery are not implemented
yet. Losing a private key means losing access to its character until an
administrative recovery workflow is available.

## Tests

```sh
make test
```

Tests cover key-to-character persistence, case-insensitive name uniqueness,
name sanitization, movement boundaries, shared snapshots, and duplicate-session
replacement.
