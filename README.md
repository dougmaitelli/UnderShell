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
| I | Open or close inventory |
| Esc | Close inventory |
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
| `ITEMS_PATH` | `./items/items.json` | JSON file containing available game items |
| `ENEMIES_PATH` | `./enemies/enemies.json` | JSON file containing available enemy types |

Example:

```sh
SSH_LISTEN_ADDR=:2022 \
DATABASE_PATH=./data/development.db \
MAPS_PATH=./maps \
ITEMS_PATH=./items/items.json \
ENEMIES_PATH=./enemies/enemies.json \
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
  "enemy_spawns": [
    {
      "enemy_id": "slime",
      "x": 1,
      "y": 1,
      "width": 4,
      "height": 3,
      "max_enemies": 3,
      "respawn_seconds": 10
    }
  ],
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

Each `enemy_spawns` entry is a rectangular roaming area. The referenced
`enemy_id` must exist in the enemy definitions. `max_enemies` controls how many
enemies owned by that spawn may be alive at once. After one is defeated, the
spawn creates one replacement every `respawn_seconds` until it reaches its cap.
Enemies choose walkable steps and cannot leave their owning spawn rectangle.

## Items

Available game items are defined in `ITEMS_PATH`. Item IDs must be unique and
use lowercase letters, numbers, underscores, or hyphens. `max_stack` must be at
least 1.

```json
{
  "items": [
    {
      "id": "health_potion",
      "name": "Health Potion",
      "description": "Restores a small amount of health.",
      "max_stack": 10
    }
  ]
}
```

The server validates and loads the full item definitions during startup.

## Enemies

Enemy types are defined in `ENEMIES_PATH`, separately from their map-owned
spawn locations. IDs follow the same format as item IDs. `visual` contains
between one and five rows of small ASCII art, with at most 15 characters per
row. Leading and trailing spaces are preserved for shaping the art.

```json
{
  "enemies": [
    {
      "id": "slime",
      "name": "Slime",
      "description": "A wobbling blob that roams the meadow.",
      "visual": [
        " .-. ",
        "(o_o)"
      ]
    }
  ]
}
```

Enemies are initially created up to each spawn's cap, appear in world snapshots,
and roam within their configured area. Enemy state is currently runtime-only.

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
