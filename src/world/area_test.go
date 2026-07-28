package world

import (
	"os"
	"path/filepath"
	"testing"

	"sshrpg/src/enemy"
	"sshrpg/src/item"
	"sshrpg/src/npc"
	"sshrpg/src/quest"
)

func TestLoadAreasValidatesInterAreaWaypoints(t *testing.T) {
	directory := t.TempDir()
	writeArea(t, directory, "one.json", `{
		"id":"one","name":"One","layout":["###","#.#","###"],
		"spawn":{"x":1,"y":1},
		"waypoints":[{"x":1,"y":1,"destination_area":"two","destination_x":1,"destination_y":1}]
	}`)
	writeArea(t, directory, "two.json", `{
		"id":"two","name":"Two","layout":["###","#.#","###"],
		"spawn":{"x":1,"y":1},"waypoints":[]
	}`)

	areas, err := LoadAreas(directory)
	if err != nil {
		t.Fatal(err)
	}
	one, ok := areas.Area("one")
	if !ok || one.Name != "One" || one.Width != 3 || one.Height != 3 {
		t.Fatalf("unexpected area: %#v", one)
	}
	two, _ := areas.Area("two")
	waypoint, ok := one.Waypoint(Point{X: 1, Y: 1})
	if !ok || waypoint.Destination != two {
		t.Fatalf("waypoint destination was not resolved: %#v", waypoint)
	}
}

func TestLoadAreasRejectsUnknownDestination(t *testing.T) {
	directory := t.TempDir()
	writeArea(t, directory, "one.json", `{
		"id":"one","name":"One","layout":["###","#.#","###"],
		"spawn":{"x":1,"y":1},
		"waypoints":[{"x":1,"y":1,"destination_area":"missing","destination_x":1,"destination_y":1}]
	}`)
	if _, err := LoadAreas(directory); err == nil {
		t.Fatal("expected unknown waypoint destination to fail validation")
	}
}

func TestBundledConfigurationIsValid(t *testing.T) {
	items, err := item.LoadItems(filepath.Join("..", "..", "items", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	enemies, err := enemy.LoadEnemies(
		filepath.Join("..", "..", "enemies", "enemies.json"), items,
	)
	if err != nil {
		t.Fatal(err)
	}
	quests, err := quest.LoadQuests(
		filepath.Join("..", "..", "quests", "quests.json"), items,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := quests.ValidateObjectives(enemies); err != nil {
		t.Fatal(err)
	}
	areas, err := LoadAreas(
		filepath.Join("..", "..", "maps"),
		References{Items: items, Enemies: enemies, Quests: quests},
	)
	if err != nil {
		t.Fatal(err)
	}
	if areas == nil {
		t.Fatal("bundled areas were not loaded")
	}
}

func TestEnemySpawnValidation(t *testing.T) {
	enemies, err := enemy.NewEnemies([]enemy.Definition{{
		ID: "slime", Name: "Slime", Visual: []string{"(s)"}, Health: 3, Experience: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAreas([]AreaDefinition{{
		ID: "one", Name: "One", Layout: []string{"#####", "#...#", "#####"},
		Spawn: Point{X: 1, Y: 1},
		EnemySpawns: []EnemySpawnDefinition{{
			EnemyID: "missing", X: 1, Y: 1, Width: 3, Height: 1,
			MaxEnemies: 2, RespawnSeconds: 5,
		}},
	}}, References{Enemies: enemies}); err == nil {
		t.Fatal("expected unknown enemy reference to fail")
	}
}

func TestDefaultSpawnMustBeWalkableInKnownArea(t *testing.T) {
	areas, err := NewAreas([]AreaDefinition{{
		ID: "one", Name: "One", Layout: []string{"###", "#.#", "###"},
		Spawn: Point{X: 1, Y: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := areas.SetDefaultSpawn("missing", Point{X: 1, Y: 1}); err == nil {
		t.Fatal("expected unknown default spawn area to fail")
	}
	if err := areas.SetDefaultSpawn("one", Point{X: 0, Y: 0}); err == nil {
		t.Fatal("expected blocked default spawn point to fail")
	}
	if err := areas.SetDefaultSpawn("one", Point{X: 1, Y: 1}); err != nil {
		t.Fatal(err)
	}
	area, point := areas.DefaultSpawn()
	canonical, _ := areas.Area("one")
	if area.ID != "one" || point != (Point{X: 1, Y: 1}) {
		t.Fatalf("unexpected default spawn: %s %#v", area.ID, point)
	}
	if area != canonical {
		t.Fatal("default spawn did not retain the canonical area reference")
	}
}

func TestShopNPCConfigurationAndItemValidation(t *testing.T) {
	items, err := item.NewItems([]item.Definition{{
		ID: "potion", Name: "Potion", Description: "Restores health.", MaxStack: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := NewAreas([]AreaDefinition{{
		ID: "market", Name: "Market",
		Layout: []string{"#####", "#...#", "#####"},
		Spawn:  Point{X: 1, Y: 1},
		NPCs: []npc.Config{{
			ID: "merchant", Name: "Mira", Type: npc.TypeShop, X: 2, Y: 1,
			Stock: []npc.ShopItemConfig{{
				ItemID: "potion", BuyPrice: 10, SellPrice: 5,
			}},
		}},
	}}, References{Items: items})
	if err != nil {
		t.Fatal(err)
	}
	area, _ := areas.Area("market")
	npc, ok := area.NPCAt(Point{X: 2, Y: 1})
	if !ok || npc.Name != "Mira" || npc.Stock[0].Item == nil ||
		npc.Stock[0].Item.Name != "Potion" {
		t.Fatalf("unexpected NPC: %#v", npc)
	}
	indexed, indexedArea, ok := areas.NPC("merchant")
	if !ok || indexed != npc || indexedArea != area {
		t.Fatal("NPC index did not retain canonical references")
	}
}

func TestShopNPCRejectsInvalidStockPrices(t *testing.T) {
	_, err := NewAreas([]AreaDefinition{{
		ID: "market", Name: "Market",
		Layout: []string{"#####", "#...#", "#####"},
		Spawn:  Point{X: 1, Y: 1},
		NPCs: []npc.Config{{
			ID: "merchant", Name: "Mira", Type: npc.TypeShop, X: 2, Y: 1,
			Stock: []npc.ShopItemConfig{{
				ItemID: "potion", BuyPrice: 5, SellPrice: 10,
			}},
		}},
	}})
	if err == nil {
		t.Fatal("expected shop sell price above buy price to fail")
	}
}

func TestAreasRequireGloballyUniqueNPCIDs(t *testing.T) {
	items, err := item.NewItems([]item.Definition{{
		ID: "potion", Name: "Potion", MaxStack: 10,
	}})
	if err != nil {
		t.Fatal(err)
	}
	definitions := make([]AreaDefinition, 2)
	for index, areaID := range []string{"one", "two"} {
		definitions[index] = AreaDefinition{
			ID: areaID, Name: areaID,
			Layout: []string{"#####", "#...#", "#####"},
			Spawn:  Point{X: 1, Y: 1},
			NPCs: []npc.Config{{
				ID: "shared_merchant", Name: "Merchant",
				Type: npc.TypeShop, X: 2, Y: 1,
				Stock: []npc.ShopItemConfig{{
					ItemID: "potion", BuyPrice: 10, SellPrice: 5,
				}},
			}},
		}
	}
	if _, err := NewAreas(definitions, References{Items: items}); err == nil {
		t.Fatal("expected duplicate NPC IDs across areas to fail")
	}
}

func writeArea(t *testing.T, directory, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
