# UnderShell

UnderShell is a configurable multiplayer RPG engine that runs over SSH and
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

## Play the public release

The public UnderShell server is available directly over SSH. No game client or
local installation is required:

```sh
ssh undershell.sh
```

On the first connection, enter a character name to begin. UnderShell associates
the character with the SSH key used to connect, so use the same key on future
connections to continue that character. If SSH has multiple keys available,
select one explicitly:

```sh
ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes undershell.sh
```

Use WASD or the arrow keys to move, and press F1 in the game to see the complete
controls. A terminal at least 46 columns wide and 14 rows tall is recommended.

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

## Staff commands

Commands can be entered in the running server's standard-input console or in
the in-game chat by an online character with the `moderator` or `admin` role.
Every command must begin with `/`.

| Command | Effect |
|---|---|
| `/exp <amount> [player]` | Grant experience |
| `/lvl <amount> [player]` | Grant levels and their skill points |
| `/item <item> <quantity> [player]` | Add an item to the inventory |
| `/tp <area> [player]` | Move a player to an area's spawn |
| `/tpTo <destination-player> [player]` | Move a player to another player |
| `/promote <player> [moderator\|admin]` | Promote a player to moderator or admin; admin-only |
| `/kick <player>` | Disconnect an online player |
| `/ban <player>` | Permanently ban an account; admin-only |
| `/unban <player>` | Remove a permanent account ban; admin-only |
| `/m <message>` | Broadcast a yellow Server message |
| `/maintenance <on\|off>` | Allow new connections from staff only |

The final `player` argument can be omitted in chat, in which case the command
targets its sender. It is required in the server console. Commands currently
target online players. Character, area, and item names are case-insensitive;
names containing spaces must be quoted:

```text
/item "Health Potion" 5 "Doug M"
/tp "Crystal Grotto" "Doug M"
```

Item and area IDs can be used instead of their display names. Moderators can
run the gameplay commands and `/kick`, but cannot kick administrators. Only
administrators and the trusted server console can use `/promote`, `/ban`, and
`/unban`. Both staff roles and the console can use `/m`; announcements appear
in every player's chat as a fully yellow `Server` line.

`/promote <player>` promotes to moderator by default. Pass `admin` explicitly
to promote a user or moderator to administrator.

Both staff roles and the console can toggle maintenance mode. Enabling it keeps
current sessions connected but refuses new user and character-creation
sessions. Moderator and administrator accounts can still connect. Maintenance
mode is runtime state and starts disabled after a server restart.

Bans are stored against the character account identified by its SSH key and
survive disconnects and server restarts. `/ban` and `/unban` accept offline
character names; the other player-targeting commands require their target to
be online.

## Run with Docker

The Compose configuration publishes host port `22` to the game server:

```sh
cp .env.example .env
# Edit .env before starting the server.
docker compose up --build -d
ssh -i ~/.ssh/id_ed25519 game.example.com
```

Port `22` must be available. Keep administrative SSH on another port or address,
or change the published port in `compose.yaml`.

Compose runs PostgreSQL alongside the game. The `database-data` volume persists
player data and the `game-data` volume persists the SSH host key. Rebuilding the
images updates the engine and bundled content without replacing either volume.
Neither named volume is a backup; regularly back up PostgreSQL and the SSH host
key separately.

Compose reads the PostgreSQL database, user, and password from `.env`. The real
file is ignored by Git; `.env.example` documents the required variables. Use a
URL-safe password because Compose also uses it to construct `DATABASE_URL`.

## Configuration

Runtime paths and listener settings are configured with environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `SSH_LISTEN_ADDR` | `:2222` | SSH listener address |
| `SSH_HOST_KEY_PATH` | `./data/ssh_host_ed25519` | Persistent server host key |
| `DATABASE_URL` | unset | PostgreSQL connection URL; takes precedence over `DATABASE_PATH` |
| `DATABASE_PATH` | `./data/game.db` | SQLite fallback for local development and tests |
| `GAME_CONFIG_PATH` | `./content/game.json` | Global game configuration |
| `AREAS_PATH` | `./content/areas` | Area definition directory |
| `OBJECTS_PATH` | `./content/objects` | Reusable map-object definition directory |
| `ITEMS_PATH` | `./content/items` | Item definition directory |
| `ENEMIES_PATH` | `./content/enemies` | Enemy definition directory |
| `QUESTS_PATH` | `./content/quests` | Quest definition directory |

For example:

```sh
SSH_LISTEN_ADDR=:2022 \
DATABASE_PATH=./data/development.db \
GAME_CONFIG_PATH=./my-game/game.json \
AREAS_PATH=./my-game/areas \
OBJECTS_PATH=./my-game/objects \
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
│   ├── objects/
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
    ├── persistence/     SQLite/PostgreSQL Bun setup and migrations
    ├── repository/      Persistent data access
    ├── ui/              Per-player terminal session and rendering
    └── sshserver/       SSH authentication and session hosting
```

The world owns shared state that is ticked or broadcast to players. A UI session
owns one player's input, menus, chat focus, and rendering. Repositories isolate
persistent storage from both.

## Persistence and identity

Local development uses `data/game.db`, while the Compose production deployment
uses PostgreSQL through `DATABASE_URL`. Both backends run the same Bun
migrations and repositories. Persistent state includes characters, roles,
positions, progression, inventories, equipment, gold, bans, and quests.

The default runtime directory also contains `ssh_host_ed25519`, the SSH server
identity presented to clients. Back up the database and host key. Replacing the
host key causes returning clients to receive a host-identity warning.

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
  "sell_price": 4,
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
Set a positive `sell_price` to let every shop buy the item. Omitting it or
setting it to `0` makes the item unsellable.
Equipment may provide non-negative `attack`, `defense`, and `vitality` stats:

```json
{
  "id": "training_blade",
  "name": "Training Blade",
  "description": "A simple practice weapon.",
  "type": "equipment",
  "sell_price": 15,
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
  "palette": "verdant",
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
  "palette": "verdant",
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
applied over that floor. Terrain tiles have consistent visual and movement
semantics:

| Tile | Terrain | Movement |
|---|---|---|
| `#` | Regional wall or rock | Blocked |
| `T` | Tree or dense vegetation | Blocked |
| `~` | Water | Blocked |
| `≈` | Lava | Blocked |
| `*` | Ice or crystal | Blocked |
| `f` | Campfire, rendered as an orange flame | Blocked |
| `W` | Stone town well | Blocked |
| `=` | Bridge, gate, or door | Walkable |
| `^` | Landmark | Walkable |
| `.` | Invisible ground | Walkable |

Other printable feature tiles are walkable and use the regional accent.
Layout rows must have equal width. `palette` controls the regional wall and
accent colors and can be `stone`, `verdant`, `redwood`, `marsh`, `coastal`,
`frost`, `ember`, `sunlit`, `crystal`, `astral`, `village`, or `iron`. An
omitted or unknown palette uses `stone`.

#### Reusable map objects

Static multi-tile assets can be defined once in `OBJECTS_PATH` and stamped
into any generated or explicit area layout. Spaces in an object layout are
transparent:

```json
{
  "id": "campfire",
  "layout": [
    " = ",
    "=f=",
    " = "
  ]
}
```

Areas place an object using the top-left coordinate of its layout:

```json
{
  "objects": [
    { "object_id": "campfire", "x": 77, "y": 33 }
  ]
}
```

Objects are applied after area `features`, in placement order. Every object
must fit entirely within the area. Unknown IDs, malformed layouts, and
out-of-bounds placements prevent the server from starting. Stamping happens
once while loading content, so map objects add no rendering or tick overhead.
The bundled definitions include campfires, braziers, a multi-tile town well,
horizontal and vertical bridges, irregular lakes, and two forest-grove sizes.

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

A shop references the items it sells and defines their purchase prices.
Resale prices come from the item definitions, so shops can buy items they do
not stock:

```json
{
  "id": "town_shop",
  "name": "Merchant",
  "type": "shop",
  "x": 12,
  "y": 18,
  "stock": [
    { "item_id": "minor_tonic", "buy_price": 10 }
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
