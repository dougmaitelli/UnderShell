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

Players are rendered as stick figures with identity markers and names overhead:
`@ Aria` identifies your yellow character, while `○ Rowan` identifies another
player in blue. `█` is a wall, and `◇` marks a waypoint to another area.

## Configuration

Configuration is supplied through environment variables.

| Variable | Default | Purpose |
|---|---|---|
| `SSH_LISTEN_ADDR` | `:2222` | Address and port used by the SSH server |
| `SSH_HOST_KEY_PATH` | `./data/ssh_host_ed25519` | Persistent Ed25519 server host-key path |
| `DATABASE_PATH` | `./data/game.db` | Persistent SQLite database path |
| `MAPS_PATH` | `./maps` | Directory containing JSON area definitions |

Example:

```sh
SSH_LISTEN_ADDR=:2022 \
DATABASE_PATH=./data/development.db \
MAPS_PATH=./maps \
make run
```

## Areas and maps

Each JSON file in `MAPS_PATH` defines one area. Layout rows must all have the
same width. A `#` tile is a wall; every other printable character is walkable.

```json
{
  "id": "meadow",
  "name": "Green Meadow",
  "layout": [
    "##########",
    "#........#",
    "##########"
  ],
  "spawn": { "x": 1, "y": 1 },
  "waypoints": [
    {
      "x": 6,
      "y": 1,
      "width": 3,
      "height": 3,
      "destination_area": "cavern",
      "destination_x": 1,
      "destination_y": 1
    }
  ]
}
```

Walking onto any tile covered by a waypoint moves the character to its
destination area and coordinate. `x` and `y` are the region's top-left corner;
`width` and `height` default to 1. Area IDs must be unique, and every waypoint
destination must refer to a loaded area and a walkable tile. The server validates
all map files during startup.

Large areas can use a compact generated layout instead of listing every row:

```json
{
  "id": "meadow",
  "name": "Green Meadow",
  "width": 192,
  "height": 64,
  "default_tile": ".",
  "border_tile": "#",
  "features": [
    { "x": 21, "y": 8, "width": 27, "height": 2, "tile": "#" }
  ],
  "spawn": { "x": 7, "y": 32 },
  "waypoints": []
}
```

Features are rectangular tile regions applied over the default tile. Generated
layouts and explicit row layouts use the same spawn and waypoint validation.

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
