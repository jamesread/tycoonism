# Data Files Specification: Resources and Buildings

Resources and buildings are defined in YAML data files. The backend and frontend must not hardcode resource or building types (e.g. no explicit "wheat" or "stone" in code). All game content is driven by these files.

## File locations

- **resources.yaml** — Defines all resources (e.g. wheat, stone). Path is configurable (e.g. `data/resources.yaml` or via `data.resources_file`).
- **buildings.yaml** — Defines all buildings (e.g. wheat farm, stone mine). Path is configurable (e.g. `data/buildings.yaml` or via `data.buildings_file`).
- **npcs.yaml** — Defines non-player characters (optional). Path is configurable via `data.npcs_file` (default `data/npcs.yaml`). If the file is missing, no NPCs are loaded.

The backend loads these at startup and validates that building definitions reference only known resource IDs. The frontend receives the same definitions via the API (see "API exposure" below).

---

## resources.yaml

Schema:

```yaml
resources:
  - id: string          # Unique identifier (e.g. "wheat", "stone"). Used in API and buildings.
  - name: string        # Human-readable name (e.g. "Wheat", "Stone").
  - icon: string        # Iconify icon name (e.g. "game-icons:wheat", "game-icons:stone-pile").
  - base_color: string  # CSS color for UI (e.g. button background). Border is computed darker.
```

- **id** is the canonical key used in protocol messages (e.g. `PlayerState.resources["wheat"]`), in building `tickResources` and `upgradeProductionFactor`, and in `SellResourceRequest.resource_id`.
- **Prices** are fully dynamic: the game recalculates a market price each tick from total supply and demand (sum of sell-order quantities vs buy-order quantities). There is no `sell_price` or `buy_price` in the file. The API returns the current market price and 3/10/50-tick average price change (like a stock market).

Example:

```yaml
resources:
  - id: wheat
    name: Wheat
    icon: game-icons:wheat
    base_color: "#c4a035"
  - id: stone
    name: Stone
    icon: game-icons:stone-pile
    base_color: "#8b8b8b"
```

---

## buildings.yaml

Schema:

```yaml
buildings:
  - id: string                    # Unique identifier (e.g. "wheat_farm", "stone_mine").
  - name: string                 # Human-readable name (e.g. "Wheat Farm", "Stone Mine").
  - icon: string                 # Iconify icon name (e.g. "game-icons:wheat", "game-icons:stone-pile").
  - base_level: int               # Level at placement (default 1). Must be >= 1.
  - tick_resources:               # Resources produced per tick at base level (level 1). Keys are resource IDs.
      <resource_id>: int          # Amount per tick at level 1.
  - upgrade_production_factor:   # Per-resource multiplier for production at higher levels (optional).
      <resource_id>: float        # When factor is 1.0 or omitted: production = base * level (linear).
                                  # Otherwise: production = base * (factor ^ (L - 1)) at level L.
  - upgrade_time_factor: float    # Upgrade duration from level L to L+1 = base_duration * (factor ^ (L - 1)).
                                  # e.g. 1.3 means each level takes 30% longer than the previous.
  - requirements:                 # Prerequisites to place this building (optional).
      - building_id: string      # Required building type ID.
        count: int               # Minimum number of that building the player must already have.
```

- **id** is the canonical key used in protocol messages (e.g. `Building.type_id`, `PlaceBuildingRequest.building_type_id`) and in `requirements.building_id`.
- **tick_resources** defines how much of each resource is produced per simulation tick when the building is at level 1. Production at level L for a resource is:  
  `base_amount * (upgrade_production_factor[resource_id] ^ (L - 1))`  
  If `upgrade_production_factor` is omitted for a resource, treat as 1.0 (linear: production = base_amount * L).
- **upgrade_time_factor** applies to the backend’s upgrade duration formula. If the global formula is `base_duration + time_step * (from_level - 1)`, then multiply the result by `upgrade_time_factor ^ (from_level - 1)` so that later levels take longer. Alternatively the backend can use a building-specific formula:  
  `duration(from_level → from_level+1) = global_base_duration * (upgrade_time_factor ^ (from_level - 1))`.
- **requirements** are checked when placing a building: the player must have at least `count` buildings of type `building_id` (any level) on the grid.

Example:

```yaml
buildings:
  - id: wheat_farm
    name: Wheat Farm
    icon: game-icons:wheat
    base_level: 1
    tick_resources:
      wheat: 3
    upgrade_production_factor:
      wheat: 1.0
    upgrade_time_factor: 1.3
    requirements: []

  - id: stone_mine
    name: Stone Mine
    icon: game-icons:stone-pile
    base_level: 1
    tick_resources:
      stone: 3
    upgrade_production_factor:
      stone: 1.0
    upgrade_time_factor: 1.3
    requirements:
      - building_id: wheat_farm
        count: 3
```

---

## npcs.yaml

Defines non-player characters (NPCs) and their AI factors. Optional: if the file is missing or path is empty, no NPCs are loaded.

Schema:

```yaml
npcs:
  - name: string        # Display name (e.g. "Alice", "Bob").
  - wealth: int         # AI factor 0–100.
  - risk: int           # AI factor 0–100.
  - resources:
      - resource_type: string   # Resource id (must exist in resources.yaml).
      - buy_eagerness: int      # 0–100.
      - sell_eagerness: int     # 0–100.
      - max_capacity: int       # Maximum quantity they will buy (≥ 0).
```

- **wealth** and **risk** are used by the game logic to drive NPC behaviour (e.g. trading decisions).
- Each NPC has a **resources** list: one entry per resource type they trade. **resource_type** must reference a resource `id` from resources.yaml. **buy_eagerness** and **sell_eagerness** are 0–100 factors. **max_capacity** is the maximum quantity that NPC will hold/buy for that resource.

Path is configurable via `data.npcs_file` (default `data/npcs.yaml`).

---

## Backend behavior

1. **Loading**  
   On startup, load `resources.yaml` and `buildings.yaml` from configured paths. If `data.npcs_file` is set, load `npcs.yaml` (missing file is allowed; no NPCs loaded). Validate: every NPC `resource_type` must be a resource `id`. Validate: every `tick_resources` and `upgrade_production_factor` key in buildings must be a resource `id`; every `requirements.building_id` must be a building `id`. Fail startup if invalid.

2. **Player state**  
   - Player resources are stored as a map: `resource_id → quantity` (e.g. `wheat: 50`, `stone: 50`). No fixed fields like `wheat`/`stone` in the stored model or protocol.
   - Starting resources for new players come from config (e.g. `game.starting_resources`) as a map of resource_id to quantity, or from a default derived from resources.yaml (e.g. 0 for all, or a configured default).

3. **Building placement**  
   - Buildings are stored with a `type_id` (string) referencing `buildings[].id`. Level at placement is the building’s `base_level`.
   - Before placing, check `requirements`: for each entry, the player must have at least `count` buildings with that `building_id`.

4. **Tick production**  
   For each building, look up its definition by `type_id`. For each resource in `tick_resources`, compute production at current level using `upgrade_production_factor` and add to the player’s resource map.

5. **Upgrade duration**  
   Use the building’s `upgrade_time_factor` in the duration formula (e.g. duration from level L to L+1 = `base_duration * (upgrade_time_factor ^ (L - 1))`).

6. **Selling**  
   `SellResourceRequest` carries `resource_id` (string). Use the current market price (dynamic) for the resource; credit coins and deduct resource quantity.

---

## API exposure

The frontend must not hardcode resource or building types. It must receive definitions from the API.

- **Option A:** Extend **WorldState** (or equivalent) with repeated/list of resource definitions and building definitions (id, name, icon, and any needed display hints). The frontend uses these to render labels, icons, and build menus.
- **Option B:** Add a separate RPC **GetGameData** (or **GetDefinitions**) that returns resources and buildings. The frontend calls it once (e.g. on load) and caches.

Each definition must include at least: **id**, **name**, **icon**. Buildings can also expose `tick_resources` and/or per-resource rates for display (e.g. "X/min") if the backend computes them.

Protocol messages must use **string IDs** for resources and buildings:

- **PlayerState**: e.g. `map<string, int64> resources` (resource_id → quantity) and per-resource rates as `map<string, int64> resources_per_min` or similar.
- **Building**: `string type_id` instead of enum `BuildingType`.
- **PlaceBuildingRequest**: `string building_type_id`.
- **SellResourceRequest**: `string resource_id` instead of enum `ResourceType`.
- **WorldState**: remove fixed `wheat_per_tick`/`stone_per_tick`; per-resource rates can be global defaults or derived from building definitions for display.

---

## Frontend behavior

- **No hardcoded wheat/stone or building types.** All labels, icons, sell buttons, and build menus are driven by the API payload (resource and building definitions).
- Use `resource.id` for keys and API calls; use `resource.name` for display; use `resource.icon` for Iconify.
- Use `building.id` for placement and keys; use `building.name` and `building.icon` for display.
- Resource amounts and rates are read from `PlayerState.resources` and `PlayerState.resources_per_min` (or equivalent) keyed by resource id.

---

## Config and defaults

- **Data file paths**: e.g. `data.resources_file`, `data.buildings_file`, `data.npcs_file` (defaults: `data/resources.yaml`, `data/buildings.yaml`, `data/npcs.yaml`). NPCs file is optional; if missing, no NPCs are loaded.
- **Starting resources**: e.g. `game.starting_resources` as a map (or list of `{ resource_id, quantity }`) so new players get configurable amounts per resource. If omitted, use 0 for all or a single default (e.g. 50 for each) from config.
- **Market prices** are computed each tick from order book supply/demand; the API exposes current price and 3/10/50-tick average price change per resource.
- **Placement cost** can remain a global `game.placement_cost` (all buildings same cost) unless a future extension adds per-building cost.
- **Upgrade cost formula** (coins) can stay global (e.g. 70% of placement cost * 1.30^(toLevel-2)); it does not need to be in the data files for this spec.
