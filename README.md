# SSH Realms

SSH Realms is a configurable multiplayer RPG engine that runs over SSH and
renders its interface in the terminal. Players need only a standard SSH client;
their public-key fingerprint is their persistent identity.

The engine provides:

- A shared, continuously ticking world with connected areas
- Server-authoritative movement, combat, enemies, drops, and respawning
- Persistent characters, inventories, equipment, progression, gold, and quests
- Shops, quest-giver NPCs, dialogue, chat, and event overlays
- JSON-defined areas, items, enemies, quests, spawns, NPCs, and waypoints
- Strict startup validation and resolved references between definitions

The bundled content is an example game built with the engine. Its topology and
progression are documented separately in [docs/world.md](docs/world.md).

## Requirements

- Go 1.26 or newer
- An OpenSSH-compatible client
- Docker, optionally, for container deployment

## Run locally

Start the server:

```sh
make run
```

The default listener is port `2222`. Create a player key and connect from
another terminal:

```sh
ssh-keygen -t ed25519 -f /tmp/sshrpg-player -N ''
ssh -p 2222 -i /tmp/sshrpg-player -o IdentitiesOnly=yes localhost
```

Each SSH key represents one character. Use another key to simulate a second
player.

To build a standalone binary:

```sh
make build
./bin/sshrpg
```

## Run with Docker

The Compose configuration publishes host port `22` to the game server:

```sh
docker compose up --build -d
ssh -i ~/.ssh/id_ed25519 game.example.com
```

Port `22` must be available. Keep administrative SSH on another port or address,
or change the published port in `compose.yaml`.

The `game-data` volume persists the SQLite database and SSH host key. Rebuilding
the image updates the engine and bundled content without replacing that runtime
data.

## Configuration

Runtime paths and listener settings are configured with environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `SSH_LISTEN_ADDR` | `:2222` | SSH listener address |
| `SSH_HOST_KEY_PATH` | `./data/ssh_host_ed25519` | Persistent server host key |
| `DATABASE_PATH` | `./data/game.db` | SQLite database |
| `GAME_CONFIG_PATH` | `./content/game.json` | Global game configuration |
| `AREAS_PATH` | `./content/areas` | Area definition directory |
| `ITEMS_PATH` | `./content/items` | Item definition directory |
| `ENEMIES_PATH` | `./content/enemies` | Enemy definition directory |
| `QUESTS_PATH` | `./content/quests` | Quest definition directory |

For example:

```sh
SSH_LISTEN_ADDR=:2022 \
DATABASE_PATH=./data/development.db \
GAME_CONFIG_PATH=./my-game/game.json \
AREAS_PATH=./my-game/areas \
ITEMS_PATH=./my-game/items \
ENEMIES_PATH=./my-game/enemies \
QUESTS_PATH=./my-game/quests \
make run
```

This makes it possible to run a custom game without modifying or replacing the
engine source.

## Project structure

```text
.
├── content/             Bundled example game
│   ├── game.json
│   ├── areas/
│   ├── items/
│   ├── enemies/
│   └── quests/
├── data/                Runtime database and SSH host key
├── docs/                Documentation for the bundled game
└── src/
    ├── config/          Environment and global configuration
    ├── content/         Shared strict JSON directory loader
    ├── domain/          Persistent domain types
    ├── item/            Item definitions and validation
    ├── enemy/           Enemy definitions and validation
    ├── quest/           Quest definitions and validation
    ├── npc/             NPC resolution and behavior definitions
    ├── world/           Shared runtime state and simulation
    ├── persistence/     SQLite/Bun setup and migrations
    ├── repository/      Persistent data access
    ├── ui/              Per-player terminal session and rendering
    └── sshserver/       SSH authentication and session hosting
```

The world owns shared state that is ticked or broadcast to players. A UI session
owns one player's input, menus, chat focus, and rendering. Repositories isolate
persistent storage from both.

## Persistence and identity

The default runtime directory contains:

- `game.db`: characters, roles, positions, progression, inventories, equipment,
  gold, and quest state.
- `ssh_host_ed25519`: the SSH server identity presented to clients.

Back up both files. Replacing the host key causes returning clients to receive a
host-identity warning.

Password authentication and SSH forwarding are disabled. The SHA-256
fingerprint of a client's public key identifies its character; the server never
receives or stores client private keys. A newer connection using the same
character key disconnects the older session.

## Content system

Each JSON file in a content directory defines exactly one object. Filenames are
for organization; references use the object's `id`.

Definitions load in dependency order:

```text
items → enemies → quests → areas
```

This lets the loader resolve IDs once and retain canonical references:

- Enemy drops resolve item definitions.
- Quest objectives resolve item definitions.
- Area enemy spawns resolve enemy definitions.
- Shop stock resolves item definitions.
- Quest-giver NPCs resolve quest definitions.
- Waypoints resolve destination areas.

The server rejects unknown JSON fields, malformed definitions, duplicate IDs,
invalid coordinates, blocked destinations, and unresolved references at
startup. Non-JSON files in the content directories are ignored.

### Create a custom game

The easiest starting point is to copy the bundled directory:

```sh
cp -R content my-game
```

Then:

1. Change `my-game/game.json` to select the global starting location.
2. Add or remove one-definition JSON files in the typed directories.
3. Keep referenced IDs consistent.
4. Start the engine with the custom paths shown in the configuration example.
5. Run `make test` after engine changes; start the server to validate custom
   content as a complete dependency graph.

### Global game configuration

`GAME_CONFIG_PATH` selects where new and defeated players spawn. It is also the
fallback when a saved character references an area that no longer exists.

```json
{
  "default_spawn": {
    "area_id": "starter_area",
    "x": 8,
    "y": 12
  }
}
```

The area must exist and the coordinate must be walkable.

### Items

Add one file per item to `ITEMS_PATH`:

```json
{
  "id": "minor_tonic",
  "name": "Minor Tonic",
  "description": "Restores a little health.",
  "type": "consumable",
  "effects": [
    { "type": "restore_health", "amount": 5 }
  ],
  "max_stack": 10
}
```

Supported item types are:

- `material`: stackable inventory content without effects or equipment stats.
- `consumable`: requires at least one supported effect. Currently,
  `restore_health` is available.
- `equipment`: requires `max_stack: 1` and an `equipment_slot`.

Equipment slots are `helmet`, `weapon`, `armor`, `boots`, `gloves`, and `legs`.
Equipment may provide non-negative `attack`, `defense`, and `vitality` stats:

```json
{
  "id": "training_blade",
  "name": "Training Blade",
  "description": "A simple practice weapon.",
  "type": "equipment",
  "equipment_slot": "weapon",
  "stats": {
    "attack": 1
  },
  "max_stack": 1
}
```

IDs must be unique and contain only lowercase letters, numbers, underscores, or
hyphens.

### Enemies

Add one file per enemy to `ENEMIES_PATH`:

```json
{
  "id": "moss_beast",
  "name": "Moss Beast",
  "description": "A slow creature covered in forest growth.",
  "health": 8,
  "damage": 2,
  "experience": 60,
  "drops": [
    { "item_id": "minor_tonic", "chance": 0.1 }
  ],
  "visual": [
    " .-. ",
    "(o_o)"
  ]
}
```

`visual` accepts one to five rows of printable ASCII, with at most 15 characters
per row. Drop chances must be greater than `0` and at most `1`. Set `damage` to
`0` for a peaceful enemy that wanders without pursuing or attacking players.

Enemy definitions describe a type. Areas decide where instances spawn.

### Quests

Add one file per quest to `QUESTS_PATH`:

```json
{
  "id": "supply_tonics",
  "name": "Supply Tonics",
  "description": "Retrieve spare tonics from creatures outside town.",
  "objective": {
    "item_id": "minor_tonic",
    "quantity": 4
  },
  "reward": {
    "gold": 25
  },
  "dialogue": {
    "offer": "Could you recover four spare tonics?",
    "in_progress": "I still need those four tonics.",
    "ready": "Excellent. Let me take those tonics.",
    "completed": "Those supplies will help a great deal."
  }
}
```

The current quest objective retrieves a quantity of one item. The objective item
must be dropped by at least one loaded enemy. Quest completion and progress are
persistent, and a character cannot complete the same quest twice.

### Areas

Add one file per area to `AREAS_PATH`. An area can use either an explicit
`layout` or a generated rectangular layout.

An explicit layout gives direct control over every tile:

```json
{
  "id": "starter_area",
  "name": "Starter Area",
  "layout": [
    "############",
    "#..........#",
    "#..........#",
    "############"
  ],
  "spawn": { "x": 1, "y": 1 },
  "waypoints": []
}
```

A generated layout is more compact for large areas:

```json
{
  "id": "starter_area",
  "name": "Starter Area",
  "width": 96,
  "height": 40,
  "border_tile": "#",
  "features": [
    { "x": 20, "y": 8, "width": 3, "height": 12, "tile": "#" }
  ],
  "spawn": { "x": 8, "y": 19 },
  "enemy_spawns": [],
  "npcs": [],
  "waypoints": []
}
```

Generated layouts use an invisible, walkable floor. `features` are rectangles
applied over that floor. A `#` tile is blocked; all other printable feature
tiles are walkable. Layout rows must have equal width.

The area-level `spawn` is a local fallback when a character has valid `area_id`
but invalid saved coordinates. It is distinct from `game.json`'s global spawn.

#### Enemy spawn regions

An enemy spawn owns a rectangular roaming region:

```json
{
  "enemy_id": "moss_beast",
  "x": 20,
  "y": 8,
  "width": 30,
  "height": 18,
  "max_enemies": 4,
  "respawn_seconds": 12
}
```

`max_enemies` limits living instances belonging to that spawn. Defeated enemies
are replaced at the configured interval until the cap is restored.

#### Waypoints

A waypoint is a walkable trigger rectangle with an explicit destination:

```json
{
  "x": 92,
  "y": 18,
  "width": 3,
  "height": 3,
  "destination_area": "next_area",
  "destination_x": 4,
  "destination_y": 19
}
```

Walking onto any trigger tile moves the player directly to the destination.
Waypoint arrival does not use the destination area's `spawn`. Define a return
waypoint when travel should be bidirectional.

#### NPCs

NPCs are positioned inside an area. IDs must be globally unique.

A shop references items and defines both trade prices:

```json
{
  "id": "town_shop",
  "name": "Merchant",
  "type": "shop",
  "x": 12,
  "y": 18,
  "stock": [
    { "item_id": "minor_tonic", "buy_price": 10, "sell_price": 5 }
  ]
}
```

A quest giver references loaded quests:

```json
{
  "id": "town_researcher",
  "name": "Researcher",
  "type": "quest_giver",
  "x": 18,
  "y": 18,
  "quests": ["supply_tonics"]
}
```

NPC coordinates must be walkable and cannot overlap the relevant spawn
positions.

## Development

Run the complete test suite:

```sh
make test
```

Run static checks:

```sh
go vet ./...
```

The repository tests engine behavior and validation rather than asserting that
specific bundled areas, enemies, items, or quests exist.
