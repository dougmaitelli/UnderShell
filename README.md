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
| X | Attack nearby enemies |
| E | Pick up nearby item drops |
| I | Open or close inventory |
| K | Open or close skills |
| T | Focus chat |
| F1 | Open or close help |
| Esc | Close inventory |
| Enter | Submit the character name |
| Ctrl+C | Disconnect |

Players are rendered as stick figures with identity markers and names overhead:
`@ Aria` identifies your yellow character, while `○ Rowan` identifies another
player in blue. `█` is a wall, and `◇` marks a waypoint to another area.

The in-game footer stays compact and points to F1. The help window contains the
complete shortcut list; close it with F1 or Escape.

## Configuration

Configuration is supplied through environment variables.

| Variable | Default | Purpose |
|---|---|---|
| `SSH_LISTEN_ADDR` | `:2222` | Address and port used by the SSH server |
| `GAME_CONFIG_PATH` | `./config/game.json` | JSON file containing global game settings |
| `SSH_HOST_KEY_PATH` | `./data/ssh_host_ed25519` | Persistent Ed25519 server host-key path |
| `DATABASE_PATH` | `./data/game.db` | Persistent SQLite database path |
| `MAPS_PATH` | `./maps` | Directory containing JSON area definitions |
| `ITEMS_PATH` | `./items/items.json` | JSON file containing available game items |
| `ENEMIES_PATH` | `./enemies/enemies.json` | JSON file containing available enemy types |

Example:

```sh
SSH_LISTEN_ADDR=:2022 \
GAME_CONFIG_PATH=./config/game.json \
DATABASE_PATH=./data/development.db \
MAPS_PATH=./maps \
ITEMS_PATH=./items/items.json \
ENEMIES_PATH=./enemies/enemies.json \
make run
```

## Game configuration

The default spawn used by both new and defeated players is defined globally in
`GAME_CONFIG_PATH`:

```json
{
  "default_spawn": {
    "area_id": "meadow",
    "x": 7,
    "y": 32
  }
}
```

The area must exist and the coordinate must be a walkable tile. Invalid game
configuration prevents the server from starting.

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
      "health": 3,
      "damage": 1,
      "experience": 25,
      "drops": [
        { "item_id": "slime_gel", "chance": 0.75 }
      ],
      "visual": [
        " .-. ",
        "(o_o)"
      ]
    }
  ]
}
```

Enemies are initially created up to each spawn's cap, appear in world snapshots,
and roam within their configured area. Each definition's `health` controls how
many attacks it survives. Pressing `X` plays a directional slash and damages
enemies within melee range; enemies at zero health die and enter their spawn's
respawn cycle.

Hostile enemies notice nearby living players, pursue the nearest one without
leaving their configured spawn area, and attack when adjacent. Enemy `damage`
is defined alongside `health`; setting it to `0` creates a peaceful enemy that
roams without pursuing or attacking players. Player health is shown in the HUD.
At zero health, the player immediately returns to the default area's starting
point with full health. This default spawn currently serves as the global new
player and death-respawn location.

Killing an enemy grants its configured `experience` to the player who dealt the
final hit. Characters begin at level 1. Reaching a new level grants one unspent
skill point, and excess experience carries toward subsequent levels:

| Current level | XP required |
|---|---:|
| 1 | 100 |
| 2 | 400 |
| 3 | 900 |
| 4 | 1,600 |

The requirement follows `100 × level²`, requiring 32,835,000 cumulative XP to
reach level 100. There is no configured maximum level. Level, experience toward
the next level, and unspent skill points are persisted with the character and
displayed in the HUD.

Press `K` to open the skills menu and spend points with keys `1–3`:

| Attribute | Effect per rank |
|---|---|
| Attack | +1 damage dealt |
| Defense | −1 damage received, down to zero |
| Vitality | +5 maximum and current health |

Attribute ranks are persistent. The skills and inventory menus are mutually
exclusive; close the open menu with its shortcut or Escape before opening the
other one.

## Event overlay

Temporary events appear in a color-coded overlay on the right side of the game.
New messages append at the bottom and push older messages upward. The overlay
shows at most the most recent 10 wrapped lines, and every event expires
independently after six seconds. It remains visible while the inventory or
skills menu is open.

Events report item pickups, XP gains, level-ups, newly earned skill points,
defeated enemies, incoming damage, player defeat, and respawning. Spending a
skill point does not create an event.

## Chat

Global chat appears in a persistent overlay at the bottom-left, opposite the
event overlay. It retains the latest 10 messages and appends new messages at the
bottom without expiration. Newly connected players receive the current recent
history.

Press `T` to focus the chat input. While focused, gameplay shortcuts are treated
as message text. Enter sends the message and returns focus to the game; Escape
cancels the draft and returns focus without sending.

Each drop entry references an item from `ITEMS_PATH`. `chance` is greater than
zero and at most one, where `1` always drops and `0.25` is a 25% chance. Dropped
items appear as the same green `◆` marker regardless of type. Press `E` within
two tiles to collect one into the persistent inventory; normal item stack limits
are respected. Enemies and uncollected ground drops are runtime-only.

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
