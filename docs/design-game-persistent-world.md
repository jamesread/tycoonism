# Design: Web-Based Persistent World Game

## 1. Overview

A web-based game with a **persistent world** that advances over a **7-day simulation cycle**. The **objective** is to **generate the most coins**. Players manage **coins** and **resources** (e.g. wheat, stone), build on a **5×5 grid**, and sell resources for coins. Buildings can be **upgraded** from level 1 to level 10, with each level requiring **progressively more time** to complete.

### 1.1 Core Loop

1. Player earns or spends **coins** (e.g. by selling resources, placing buildings, upgrading).
2. Player places and **upgrades buildings** on their 5×5 grid; buildings **produce resources** (wheat from wheat farms, stone from stone mines).
3. Player **sells resources** for coins.
4. The **world simulation** runs over 7 days (real or game time).
5. Simulation outcomes affect economy and possibly world state; player continues building, upgrading, and selling to maximize coins.

### 1.2 Design Goals

- **Persistent**: World and player state survive restarts and are stored server-side.
- **Simulated**: A discrete 7-day cycle drives world progression.
- **Web-first**: All interaction via browser; backend holds authority over state and simulation.

---

## 2. Architecture

Alignment with the project layout and AGENTS.md:

| Layer        | Location           | Technology / Approach                                      |
|-------------|--------------------|------------------------------------------------------------|
| **Frontend** | `frontend/`        | Vue 3, Vite, Vue Router, picocrank (femtocrank styling)    |
| **Backend**  | `service/`         | Go, logrus, koanf, connectrpc, httpauthshim               |
| **Protocol** | `protocol/`        | Protocol Buffers v3, buf, connectrpc                      |
| **Integration tests** | `integration-tests/` | Mocha, selenium-webdriver, backend config + lifecycle |

- **Build / run**: Make; `make` builds everything; subdirectories use `make -wC frontend` etc.
- **API**: connectrpc (gRPC) for game and simulation services; HTTP only where needed (e.g. static assets, auth shim).

---

## 3. Persistent World and 7-Day Simulation

### 3.1 World Model

- **World** = shared simulation state that advances in **discrete 7-day cycles**.
- Each cycle has a **phase** or **tick** (e.g. day 1..7) and a **cycle ID** or epoch for persistence.
- Simulation runs **server-side only**; frontend displays results and sends player actions.

### 3.2 Simulation Trigger

Choose one (or a hybrid):

- **Time-based**: Real-world timer (e.g. one game day per N minutes/hours); cron or scheduler advances days.
- **Tick-based**: Backend runs a simulation tick (e.g. on demand or on a fixed interval) and advances day 1→7, then resets to day 1 and increments cycle.
- **Event-based**: Certain actions or thresholds trigger “end of day” or “end of cycle.”

Recommendation: **tick-based** with a configurable interval (e.g. via koanf) so that integration tests can drive the simulation deterministically.

### 3.3 What the Simulation Affects

- **Coins**: e.g. income from buildings, events, or penalties per day/cycle.
- **World state**: e.g. global modifiers, events, or resources that influence building output or upgrade costs.
- **Cycle boundaries**: Optional “season” or “epoch” rewards, leaderboards, or resets (e.g. soft reset of some world state every 7 days).

Persistence: store at least **current day (1–7), cycle ID, and last tick timestamp** so that restarts and catch-up logic are consistent.

---

## 4. Player Economy: Coins and Resources

- **Objective**: Generate the **most coins** (e.g. win condition or leaderboard is coin total).
- **Coins** are the primary **currency**.
- **Resources** (e.g. **wheat**, **stone**) are produced by buildings and can be **sold** for coins.
- **Coin sources**: Starting balance, **selling resources** (wheat, stone), simulation rewards, one-time or recurring events.
- **Coin sinks**: Placing buildings, upgrading buildings, optional other purchases.
- All balance and resource changes are **authoritative on the backend**; frontend only displays and sends intent (e.g. "sell wheat", “upgrade building at (x,y)”).

Persistence: store **coins and resource quantities per player** (and optionally a transaction log for auditing).

---

## 5. Grid and Buildings

### 5.1 Grid

- **5×5** grid per player (25 cells).
- Each cell is either **empty** or occupied by **exactly one building**.
- Coordinates: **(0,0)** to **(4,4)** (row, col) or (x, y) — define once in protocol and docs.

### 5.2 Buildings

- **Building**: has a **type**, a **level** (1–10), and produces a **resource** (or none). Building types include:
  - **Wheat farm**: produces **wheat** (generated over time per simulation tick/day).
  - **Stone mine**: produces **stone** (generated over time per simulation tick/day).
- **Level 1**: placement only (no “upgrade from 0”); levels 2–10 are upgrades. Higher levels typically produce more resource per tick.
- **Placement**: player pays a cost in coins and chooses an empty cell; server validates and deducts coins, then creates the building at level 1.
- **Produced resources** accumulate in the player's inventory and can be **sold** for coins (prices or rates defined by game config or simulation).

### 5.3 Upgrades (Level 2–10)

- **Cost**: Each upgrade level has a **coin cost** (e.g. increasing with level).
- **Duration**: Each level takes **progressively more time** (e.g. level 2 = 1 min, level 3 = 2 min, …, level 10 = long). Formula examples:
  - Linear: `baseTime + (level - 1) * increment`
  - Exponential: `baseTime * factor^(level - 1)`
- Upgrades are **asynchronous**: player starts upgrade, server records **completion time**; when world time (or a tick) passes that time, server sets building to the new level.
- Only **one upgrade per building** at a time; optionally one upgrade per player at a time (design choice).

Persistence: store **building type, level, cell (x,y), and optional upgrade queue** (e.g. target level, completion time).

---

## 6. Upgrade Time Progression

Suggested curve (tunable via config):

- **Level 1→2**: base duration (e.g. 30 seconds for testing, 5 minutes for production).
- **Level k→k+1**: `duration(k) = base * k^exponent` or `base + (k-1)*step`.
- **Max level**: 10; no upgrade from 10.

Example (linear step):

| Upgrade   | Duration (example) |
|----------|---------------------|
| 1 → 2    | 1 min               |
| 2 → 3    | 2 min               |
| 3 → 4    | 3 min               |
| …        | …                   |
| 9 → 10   | 9 min               |

Example (exponential, base=1 min, factor=1.5):

- 1→2: 1 min, 2→3: 1.5 min, 3→4: 2.25 min, … (cap or round as needed).

Config (e.g. koanf): `base_upgrade_duration`, `upgrade_time_factor` or `upgrade_time_step`, `max_level=10`.

---

## 7. Frontend (Web Interface)

### 7.1 Responsibilities

- **Auth**: Log in / session (via httpauthshim or existing auth); send identity on each request.
- **Grid view**: Render 5×5 grid; show empty cells and buildings (type + level).
- **Actions**: Place building (wheat farm or stone mine + cell), start upgrade (choose building), **sell resources** (wheat, stone) for coins.
- **Feedback**: Current coins, **wheat and stone quantities**, ongoing upgrade countdowns, simulation day/cycle.
- **Simulation display**: Show current day (1–7), cycle number, and optionally next tick time.

### 7.2 Technology

- **Vue 3 + Vite** in `frontend/`.
- **Vue Router** for routes (e.g. `/`, `/game`, `/leaderboard`).
- **picocrank** for shared components and utilities; **femtocrank** for styling; minimal custom CSS.
- **connectrpc** client (generated from protocol) to call backend services (e.g. GetPlayerState, PlaceBuilding, StartUpgrade, SellResource, GetWorldState).

### 7.3 UX Considerations

- Clear distinction between **empty**, **building**, and **upgrading** states.
- Countdown or progress for upgrades (e.g. “Level 4 in 2m 30s”).
- Disable or grey out actions when insufficient coins or when upgrade is in progress.

---

## 8. Protocol (connectrpc + buf)

Suggested service/message split (all in `protocol/`, proto3):

### 8.1 World / Simulation

- **WorldState**: `current_day` (1–7), `cycle_id`, `next_tick_at` (optional).
- **GetWorldState** (unary): returns current world state for display.

### 8.2 Player

- **PlayerState**: `player_id`, `coins`, `wheat`, `stone` (resource quantities), `grid` (e.g. repeated `Building` or 5×5 encoded).
- **Building**: `cell_x`, `cell_y`, `type` (e.g. wheat_farm, stone_mine), `level`, `upgrade_finishes_at` (optional; 0 or omit if not upgrading).
- **GetPlayerState** (unary): returns coins, resources, and full grid.

### 8.3 Actions

- **PlaceBuildingRequest**: `building_type` (e.g. wheat_farm, stone_mine), `cell_x`, `cell_y`. Response: success/failure, updated PlayerState or error.
- **StartUpgradeRequest**: `cell_x`, `cell_y`. Response: success/failure (e.g. already max level, already upgrading, insufficient coins), updated state.
- **SellResourceRequest**: resource type (wheat and/or stone) and quantity (or "sell all"). Response: success/failure, updated coins and resource quantities.
- **CancelUpgradeRequest** (optional): `cell_x`, `cell_y`.

All mutable RPCs are **authoritative**: server validates coins, resources, grid bounds, and upgrade rules, then persists and returns new state.

---

## 9. Data Model and Persistence

### 9.1 Entities

- **World**: `cycle_id`, `current_day`, `last_tick_at`, config snapshot if needed.
- **Player**: `id`, `coins`, `wheat`, `stone`, `created_at`, auth linkage.
- **Building**: `player_id`, `cell_x`, `cell_y`, `type` (e.g. wheat_farm, stone_mine), `level`, `upgrade_target_level`, `upgrade_finishes_at` (nullable).

### 9.2 Storage

- Backend in `service/` chooses store (e.g. SQLite, PostgreSQL). No schema in this design doc; define in a separate migration/schema doc.
- Ensure **unique (player_id, cell_x, cell_y)** for buildings and **idempotent or guarded** placement/upgrade so concurrent requests don’t double-spend or double-place.

### 9.3 Simulation Tick

On tick:

1. Advance **world time** (e.g. `last_tick_at` += 1 day; if day was 7, set day=1 and increment `cycle_id`).
2. **Apply completed upgrades**: for each building with `upgrade_finishes_at <= now`, set `level = upgrade_target_level`, clear upgrade fields.
3. **Generate resources**: for each building, add produced resource to player (e.g. wheat from wheat farms, stone from stone mines; amount depends on building type and level).
4. **Apply day/cycle effects**: e.g. events or price changes. Persist updated resources and world state. Coins increase only when the player **sells** resources (via SellResource RPC).

---

## 10. Testing Strategy

- **Unit tests** (Go in `service/`): grid logic (placement, bounds), upgrade duration calculation, coin deduction, simulation tick (day advance, upgrade completion).
- **Integration tests** (`integration-tests/tests/`): Mocha + selenium-webdriver; start/stop backend with `-configdir`; test login, load grid, place building, start upgrade, and optionally wait for upgrade or advance simulation.
- **Protocol**: Build and lint with buf; ensure generated connectrpc code is used by both frontend and backend.

---

## 11. Configuration (koanf)

Suggested keys (all optional with sane defaults):

- **Simulation**
  - `simulation.tick_interval` (e.g. 1m, 1h per game day).
  - `simulation.days_per_cycle` (7).
- **Economy**
  - `game.starting_coins`
  - `game.placement_cost_per_type` or base + per-type (e.g. wheat_farm, stone_mine).
  - `game.sell_price_wheat`, `game.sell_price_stone` (or per-cycle/dynamic pricing).
  - Resource generation per building type/level (e.g. wheat per tick for wheat_farm, stone per tick for stone_mine).
- **Upgrades**
  - `game.upgrade.base_duration`, `game.upgrade.time_factor` or `time_step`, `game.upgrade.max_level` (10).
  - `game.upgrade.cost_base` or cost table per level.

---

## 12. Security and Fairness

- **Auth**: All state-changing RPCs require authenticated player; backend resolves player_id from session/token (httpauthshim).
- **Validation**: Bounds check (0–4), “cell empty” on place, “building exists and not upgrading” on start upgrade, “level < 10” for upgrade, coin checks before deducting.
- **Idempotency**: Optional idempotency keys for PlaceBuilding / StartUpgrade to avoid double actions on retry.

---

## 13. Future Extensions (Out of Scope for v1)

- Additional building types or resources beyond wheat farm and stone mine.
- World events that modify income or costs during certain days.
- Leaderboards or rankings at end of 7-day cycle.
- Social features (e.g. visit another player’s grid, read-only).
- Real-time push (e.g. WebSocket or SSE) for simulation tick and upgrade completion instead of polling.

---

## 14. Summary

| Concept            | Design choice                                                                 |
|--------------------|-------------------------------------------------------------------------------|
| **Objective**      | Generate the most coins                                                      |
| **World**          | Persistent, server-authoritative, 7-day simulation cycle                     |
| **Simulation**     | Tick-based; advances day 1→7 then new cycle                                  |
| **Player**         | Coins + wheat + stone + 5×5 grid; state in backend                           |
| **Buildings**      | Wheat farm (produces wheat), stone mine (produces stone); one per cell; level 1–10 |
| **Economy**        | Buildings generate resources; player sells resources for coins                |
| **Upgrade time**   | Progressive per level (configurable formula)                                 |
| **Frontend**       | Vue + Vite + Router + picocrank; connectrpc client         |
| **Backend**        | Go in `service/`; connectrpc; koanf; logrus                 |
| **Protocol**       | Proto3 in `protocol/`; buf + connectrpc                    |
| **Tests**          | Unit (Go), integration (Mocha + selenium), protocol (buf)  |

This design is ready to be broken down into implementation tasks (protocol first, then backend persistence and simulation, then frontend, then integration tests).
