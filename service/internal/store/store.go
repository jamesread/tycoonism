package store

import (
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"webfarm/internal/config"
	"webfarm/internal/game"
)

const GridSize = 5

type World struct {
	CycleID    int64
	CurrentDay int32
	NextTickAt time.Time
	mu         sync.RWMutex
}

type BuildingRow struct {
	CellX               int32
	CellY               int32
	TypeID              string
	Level               int32
	UpgradeTargetLevel  int32
	UpgradeFinishesAt   time.Time
	SellingFinishesAt   time.Time
}

type Player struct {
	ID        string
	Coins     int64
	Resources map[string]int64
	Buildings []BuildingRow
	mu        sync.RWMutex
}

// OrderType is "sell" or "buy" for marketplace orders.
const (
	OrderTypeSell = "sell"
	OrderTypeBuy  = "buy"
)

// Order is a single marketplace order (sell or buy).
type Order struct {
	OrderID      string
	PlayerID     string
	OrderType    string // OrderTypeSell or OrderTypeBuy
	ResourceID   string
	Quantity     int64
	PricePerUnit int64
	CreatedAt    time.Time
}

// Loan is a player's debt to the bank. Interest is added to balance each tick.
type Loan struct {
	LoanID         string
	PlayerID       string
	Balance        int64     // remaining amount owed
	InterestRate   float64   // applied each tick (e.g. 0.01 = 1%); added to balance
	CreatedAt      time.Time
	OriginalAmount int64     // amount originally borrowed
}

// ResourceMarket holds dynamic price and history for one resource (supply/demand from orders).
const marketPriceHistoryLen = 50
const defaultMarketPrice = 10
const marketPriceSensitivity = 0.2
const marketPriceMin = 1
const marketPriceMax = 1_000_000_000

type ResourceMarket struct {
	CurrentPrice int64   // latest price (recalculated each tick)
	History      []int64 // last N prices for average change (oldest first, cap at marketPriceHistoryLen)
}

type Store struct {
	world   World
	players map[string]*Player
	orders  map[string]*Order
	loans   map[string]*Loan
	market  map[string]*ResourceMarket // resourceID -> price state
	mu      sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		world: World{
			CurrentDay: 1,
			NextTickAt: time.Now(),
		},
		players: make(map[string]*Player),
		orders:  make(map[string]*Order),
		loans:   make(map[string]*Loan),
		market:  make(map[string]*ResourceMarket),
	}
}

func (s *Store) GetOrCreatePlayer(playerID string, startingCoins int64, startingResources map[string]int64) *Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.players[playerID]; ok {
		return p
	}
	res := make(map[string]int64)
	for k, v := range startingResources {
		res[k] = v
	}
	p := &Player{
		ID:        playerID,
		Coins:     startingCoins,
		Resources: res,
	}
	s.players[playerID] = p
	return p
}

const npcPlayerIDPrefix = "npc:"

// NPCPlayerID returns the store player ID for an NPC (used for marketplace orders).
func NPCPlayerID(npcName string) string {
	return npcPlayerIDPrefix + npcName
}

// IsNPCPlayerID reports whether the player ID is an NPC (and should be excluded from leaderboard, etc.).
func IsNPCPlayerID(playerID string) bool {
	return len(playerID) >= len(npcPlayerIDPrefix) && playerID[:len(npcPlayerIDPrefix)] == npcPlayerIDPrefix
}

// GetOrCreateNPCPlayer returns the player entity for an NPC, creating it with the given coins and no resources if missing.
func (s *Store) GetOrCreateNPCPlayer(npcName string, initialCoins int64) *Player {
	id := NPCPlayerID(npcName)
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.players[id]; ok {
		return p
	}
	p := &Player{
		ID:        id,
		Coins:     initialCoins,
		Resources: make(map[string]int64),
	}
	s.players[id] = p
	return p
}

func (s *Store) GetPlayer(playerID string) *Player {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.players[playerID]
}

func (s *Store) AllPlayers() []*Player {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Player, 0, len(s.players))
	for _, p := range s.players {
		out = append(out, p)
	}
	return out
}

func (s *Store) World() *World {
	return &s.world
}

// AddOrder adds a marketplace order. Caller must have already reserved resource/coins on the player.
func (s *Store) AddOrder(o *Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.orders == nil {
		s.orders = make(map[string]*Order)
	}
	s.orders[o.OrderID] = o
}

// OrdersByPlayer returns the UUIDs of the player's open sell and buy orders.
func (s *Store) OrdersByPlayer(playerID string) (sellOrderIDs, buyOrderIDs []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, o := range s.orders {
		if o == nil || o.PlayerID != playerID {
			continue
		}
		if o.OrderType == OrderTypeSell {
			sellOrderIDs = append(sellOrderIDs, o.OrderID)
		} else if o.OrderType == OrderTypeBuy {
			buyOrderIDs = append(buyOrderIDs, o.OrderID)
		}
	}
	return sellOrderIDs, buyOrderIDs
}

// GetOrder returns the order by ID or nil.
func (s *Store) GetOrder(orderID string) *Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[orderID]
	if !ok || o == nil {
		return nil
	}
	return &Order{
		OrderID:      o.OrderID,
		PlayerID:     o.PlayerID,
		OrderType:    o.OrderType,
		ResourceID:   o.ResourceID,
		Quantity:     o.Quantity,
		PricePerUnit: o.PricePerUnit,
		CreatedAt:    o.CreatedAt,
	}
}

// GetSellOrders returns all sell orders, optionally filtered by resourceID (empty = all).
func (s *Store) GetSellOrders(resourceID string) []*Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Order, 0)
	for _, o := range s.orders {
		if o != nil && o.OrderType == OrderTypeSell {
			if resourceID == "" || o.ResourceID == resourceID {
				out = append(out, &Order{
					OrderID:      o.OrderID,
					PlayerID:     o.PlayerID,
					OrderType:    o.OrderType,
					ResourceID:   o.ResourceID,
					Quantity:     o.Quantity,
					PricePerUnit: o.PricePerUnit,
					CreatedAt:    o.CreatedAt,
				})
			}
		}
	}
	return out
}

// GetBuyOrders returns all buy orders, optionally filtered by resourceID (empty = all).
func (s *Store) GetBuyOrders(resourceID string) []*Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Order, 0)
	for _, o := range s.orders {
		if o != nil && o.OrderType == OrderTypeBuy {
			if resourceID == "" || o.ResourceID == resourceID {
				out = append(out, &Order{
					OrderID:      o.OrderID,
					PlayerID:     o.PlayerID,
					OrderType:    o.OrderType,
					ResourceID:   o.ResourceID,
					Quantity:     o.Quantity,
					PricePerUnit: o.PricePerUnit,
					CreatedAt:    o.CreatedAt,
				})
			}
		}
	}
	return out
}

// RemoveOrder removes and returns the order, or nil if not found.
func (s *Store) RemoveOrder(orderID string) *Order {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[orderID]
	if !ok {
		return nil
	}
	delete(s.orders, orderID)
	return o
}

func (s *Store) getOrCreateMarket(resourceID string) *ResourceMarket {
	if s.market == nil {
		s.market = make(map[string]*ResourceMarket)
	}
	if m, ok := s.market[resourceID]; ok {
		return m
	}
	m := &ResourceMarket{CurrentPrice: defaultMarketPrice, History: []int64{defaultMarketPrice}}
	s.market[resourceID] = m
	return m
}

// supplyDemandForResource returns total quantity in sell orders (supply) and buy orders (demand) for the resource. Caller must hold at least s.mu.RLock.
func (s *Store) supplyDemandForResource(resourceID string) (supply, demand int64) {
	for _, o := range s.orders {
		if o == nil || o.ResourceID != resourceID {
			continue
		}
		if o.OrderType == OrderTypeSell {
			supply += o.Quantity
		} else if o.OrderType == OrderTypeBuy {
			demand += o.Quantity
		}
	}
	return supply, demand
}

// updateMarketPrices recalculates current price from supply/demand for each resource and appends to history. Call with resource IDs from config.
func (s *Store) updateMarketPrices(resourceIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range resourceIDs {
		m := s.getOrCreateMarket(id)
		supply, demand := s.supplyDemandForResource(id)
		total := supply + demand + 1
		imbalance := demand - supply
		changeRatio := float64(imbalance) / float64(total)
		delta := marketPriceSensitivity * changeRatio
		next := int64(float64(m.CurrentPrice) * (1 + delta))
		if next < marketPriceMin {
			next = marketPriceMin
		}
		if next > marketPriceMax {
			next = marketPriceMax
		}
		m.CurrentPrice = next
		m.History = append(m.History, next)
		if len(m.History) > marketPriceHistoryLen {
			m.History = m.History[len(m.History)-marketPriceHistoryLen:]
		}
	}
}

// GetMarketPrice returns the current market price for the resource (dynamic, from supply/demand).
func (s *Store) GetMarketPrice(resourceID string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.market[resourceID]
	if m == nil {
		return defaultMarketPrice
	}
	return m.CurrentPrice
}

// GetMarketPriceHistory returns the last n tick prices for the resource (oldest first). May return fewer than n if history is shorter.
func (s *Store) GetMarketPriceHistory(resourceID string, n int) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.market[resourceID]
	if m == nil || len(m.History) == 0 {
		return nil
	}
	hist := m.History
	if n <= 0 || n >= len(hist) {
		out := make([]int64, len(hist))
		copy(out, hist)
		return out
	}
	start := len(hist) - n
	out := make([]int64, n)
	copy(out, hist[start:])
	return out
}

// MarketPriceAverages holds the current price and average price change over 3, 10, and 50 ticks.
type MarketPriceAverages struct {
	CurrentPrice   int64
	AvgChange3Tick  float64
	AvgChange10Tick float64
	AvgChange50Tick float64
}

// GetMarketPriceAverages returns current price and average increase/decrease over the last 3, 10, and 50 ticks.
func (s *Store) GetMarketPriceAverages(resourceID string) MarketPriceAverages {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := MarketPriceAverages{CurrentPrice: defaultMarketPrice}
	m := s.market[resourceID]
	if m == nil || len(m.History) < 2 {
		return out
	}
	out.CurrentPrice = m.CurrentPrice
	hist := m.History
	avgChange := func(n int) float64 {
		if n <= 0 || len(hist) < 2 {
			return 0
		}
		if n > len(hist)-1 {
			n = len(hist) - 1
		}
		start := len(hist) - 1 - n
		if start < 0 {
			start = 0
		}
		var sum float64
		count := 0
		for i := start; i < len(hist)-1; i++ {
			prev := float64(hist[i])
			if prev == 0 {
				continue
			}
			sum += (float64(hist[i+1]) - prev) / prev
			count++
		}
		if count == 0 {
			return 0
		}
		return sum / float64(count)
	}
	out.AvgChange3Tick = avgChange(3)
	out.AvgChange10Tick = avgChange(10)
	out.AvgChange50Tick = avgChange(50)
	return out
}

// AddLoan adds a new loan and gives the player the coins. Caller must not hold s.mu.
func (s *Store) AddLoan(loan *Loan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loans == nil {
		s.loans = make(map[string]*Loan)
	}
	s.loans[loan.LoanID] = loan
	if p := s.players[loan.PlayerID]; p != nil {
		p.AddCoins(loan.Balance)
	}
}

// GetLoansByPlayer returns all loans for the player (copy).
func (s *Store) GetLoansByPlayer(playerID string) []*Loan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Loan, 0)
	for _, l := range s.loans {
		if l != nil && l.PlayerID == playerID {
			out = append(out, &Loan{
				LoanID:         l.LoanID,
				PlayerID:       l.PlayerID,
				Balance:        l.Balance,
				InterestRate:   l.InterestRate,
				CreatedAt:      l.CreatedAt,
				OriginalAmount: l.OriginalAmount,
			})
		}
	}
	return out
}

// LoanCountByPlayer returns the number of loans the player has.
func (s *Store) LoanCountByPlayer(playerID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, l := range s.loans {
		if l != nil && l.PlayerID == playerID {
			n++
		}
	}
	return n
}

// GetLoan returns a copy of the loan by ID or nil.
func (s *Store) GetLoan(loanID string) *Loan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.loans[loanID]
	if !ok || l == nil {
		return nil
	}
	return &Loan{
		LoanID:         l.LoanID,
		PlayerID:       l.PlayerID,
		Balance:        l.Balance,
		InterestRate:   l.InterestRate,
		CreatedAt:      l.CreatedAt,
		OriginalAmount: l.OriginalAmount,
	}
}

// PayOffLoan reduces the loan balance by amount (deducted from player coins). Returns true if loan was paid off and removed.
func (s *Store) PayOffLoan(loanID string, playerID string, amount int64) (paidOff bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.loans[loanID]
	if !ok || l == nil {
		return false, nil
	}
	if l.PlayerID != playerID {
		return false, nil
	}
	if amount <= 0 {
		return false, nil
	}
	if amount > l.Balance {
		amount = l.Balance
	}
	p := s.players[playerID]
	if p == nil {
		return false, nil
	}
	if !p.SpendCoins(amount) {
		return false, nil
	}
	l.Balance -= amount
	if l.Balance <= 0 {
		delete(s.loans, loanID)
		return true, nil
	}
	return false, nil
}

func (p *Player) BuildingAt(x, y int32) *BuildingRow {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i := range p.Buildings {
		if p.Buildings[i].CellX == x && p.Buildings[i].CellY == y {
			return &p.Buildings[i]
		}
	}
	return nil
}

func (p *Player) HasBuildingAt(x, y int32) bool {
	return p.BuildingAt(x, y) != nil
}

func (p *Player) AddBuilding(b BuildingRow) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Buildings = append(p.Buildings, b)
}

func (p *Player) RemoveBuilding(x, y int32) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Buildings {
		if p.Buildings[i].CellX == x && p.Buildings[i].CellY == y {
			p.Buildings = append(p.Buildings[:i], p.Buildings[i+1:]...)
			return true
		}
	}
	return false
}

func (p *Player) StartSell(x, y int32, finishesAt time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Buildings {
		if p.Buildings[i].CellX == x && p.Buildings[i].CellY == y {
			p.Buildings[i].SellingFinishesAt = finishesAt
			return true
		}
	}
	return false
}

func (p *Player) AllBuildings() []BuildingRow {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]BuildingRow, len(p.Buildings))
	copy(out, p.Buildings)
	return out
}

func (p *Player) AddCoins(delta int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Coins += delta
}

func (p *Player) SpendCoins(amount int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if amount < 0 || p.Coins < amount {
		return false
	}
	p.Coins -= amount
	return true
}

func (p *Player) GetCoins() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Coins
}

func (p *Player) GetResource(id string) int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Resources[id]
}

func (p *Player) CopyResources() map[string]int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]int64, len(p.Resources))
	for k, v := range p.Resources {
		out[k] = v
	}
	return out
}

func (p *Player) SellResource(id string, quantity int64) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if quantity <= 0 {
		quantity = p.Resources[id]
	}
	if quantity <= 0 || quantity > p.Resources[id] {
		return 0
	}
	p.Resources[id] -= quantity
	return quantity
}

func (p *Player) AddResource(id string, delta int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Resources[id] += delta
}

func (p *Player) SetBuildingLevel(cellX, cellY int32, level int32, finishesAt time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Buildings {
		if p.Buildings[i].CellX == cellX && p.Buildings[i].CellY == cellY {
			p.Buildings[i].Level = level
			p.Buildings[i].UpgradeTargetLevel = 0
			p.Buildings[i].UpgradeFinishesAt = time.Time{}
			return
		}
	}
}

func (p *Player) StartUpgrade(cellX, cellY int32, targetLevel int32, finishesAt time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Buildings {
		if p.Buildings[i].CellX == cellX && p.Buildings[i].CellY == cellY {
			p.Buildings[i].UpgradeTargetLevel = targetLevel
			p.Buildings[i].UpgradeFinishesAt = finishesAt
			return
		}
	}
}

func (p *Player) CancelUpgrade(cellX, cellY int32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Buildings {
		if p.Buildings[i].CellX == cellX && p.Buildings[i].CellY == cellY {
			p.Buildings[i].UpgradeTargetLevel = 0
			p.Buildings[i].UpgradeFinishesAt = time.Time{}
			return
		}
	}
}

func (w *World) AdvanceTick(daysPerCycle int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.CurrentDay++
	if w.CurrentDay > int32(daysPerCycle) {
		w.CurrentDay = 1
		w.CycleID++
	}
}

func (w *World) SetNextTickAt(t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.NextTickAt = t
}

func (w *World) State() (cycleID int64, currentDay int32, nextTickAt time.Time) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.CycleID, w.CurrentDay, w.NextTickAt
}

func (s *Store) RunTick(now time.Time, cfg *config.Config) {
	for _, p := range s.AllPlayers() {
		p.mu.Lock()
		var toRemove []int
		var totalRefund int64
		for i := range p.Buildings {
			b := &p.Buildings[i]
			if !b.UpgradeFinishesAt.IsZero() && !now.Before(b.UpgradeFinishesAt) {
				b.Level = b.UpgradeTargetLevel
				b.UpgradeTargetLevel = 0
				b.UpgradeFinishesAt = time.Time{}
			}
			if !b.SellingFinishesAt.IsZero() && !now.Before(b.SellingFinishesAt) {
				toRemove = append(toRemove, i)
				totalRefund += game.SellRefund(cfg, b.Level)
			} else {
				produced := game.ResourceProducedByBuilding(b.TypeID, b.Level, cfg.Buildings)
				for resID, amount := range produced {
					p.Resources[resID] += amount
				}
			}
		}
		for i := len(toRemove) - 1; i >= 0; i-- {
			idx := toRemove[i]
			p.Buildings = append(p.Buildings[:idx], p.Buildings[idx+1:]...)
		}
		p.Coins += totalRefund
		p.mu.Unlock()
	}
	s.applyLoanInterest()
	s.world.AdvanceTick(cfg.DaysPerCycle)
	s.world.SetNextTickAt(now.Add(cfg.TickInterval))
	var resourceIDs []string
	for _, r := range cfg.Resources {
		resourceIDs = append(resourceIDs, r.ID)
	}
	s.updateMarketPrices(resourceIDs)
}

// applyLoanInterest deducts interest from each player's coins per loan; unpaid interest is added to the loan balance.
func (s *Store) applyLoanInterest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.loans {
		if l == nil || l.Balance <= 0 {
			continue
		}
		p := s.players[l.PlayerID]
		if p == nil {
			continue
		}
		interest := int64(float64(l.Balance) * l.InterestRate)
		if interest <= 0 {
			continue
		}
		p.mu.Lock()
		coins := p.Coins
		pay := interest
		if coins < pay {
			pay = coins
		}
		p.Coins -= pay
		p.mu.Unlock()
		unpaid := interest - pay
		l.Balance += unpaid
	}
}

// PersistedWorld is the YAML-serializable world state.
type PersistedWorld struct {
	CycleID    int64     `yaml:"cycle_id"`
	CurrentDay int32     `yaml:"current_day"`
	NextTickAt time.Time `yaml:"next_tick_at"`
}

// PersistedBuilding is the YAML-serializable building row.
type PersistedBuilding struct {
	CellX               int32     `yaml:"cell_x"`
	CellY               int32     `yaml:"cell_y"`
	Type                int32     `yaml:"type,omitempty"`   // legacy enum: 1=wheat_farm, 2=stone_mine
	TypeID              string    `yaml:"type_id,omitempty"`
	Level               int32     `yaml:"level"`
	UpgradeTargetLevel  int32     `yaml:"upgrade_target_level"`
	UpgradeFinishesAt   time.Time `yaml:"upgrade_finishes_at"`
	SellingFinishesAt   time.Time `yaml:"selling_finishes_at"`
}

// PersistedPlayer is the YAML-serializable player state.
type PersistedPlayer struct {
	ID        string              `yaml:"id"`
	Coins     int64               `yaml:"coins"`
	Wheat     int64               `yaml:"wheat,omitempty"`   // legacy
	Stone     int64               `yaml:"stone,omitempty"`   // legacy
	Resources map[string]int64    `yaml:"resources,omitempty"`
	Buildings []PersistedBuilding `yaml:"buildings"`
}

// PersistedOrder is the YAML-serializable marketplace order.
type PersistedOrder struct {
	OrderID      string    `yaml:"order_id"`
	PlayerID     string    `yaml:"player_id"`
	OrderType    string    `yaml:"order_type"`
	ResourceID   string    `yaml:"resource_id"`
	Quantity     int64     `yaml:"quantity"`
	PricePerUnit int64     `yaml:"price_per_unit"`
	CreatedAt    time.Time `yaml:"created_at"`
}

// PersistedLoan is the YAML-serializable loan.
type PersistedLoan struct {
	LoanID         string    `yaml:"loan_id"`
	PlayerID       string    `yaml:"player_id"`
	Balance        int64     `yaml:"balance"`
	InterestRate   float64   `yaml:"interest_rate"`
	CreatedAt      time.Time `yaml:"created_at"`
	OriginalAmount int64     `yaml:"original_amount"`
}

// PersistedMarketResource is the serializable market state for one resource.
type PersistedMarketResource struct {
	ResourceID    string  `yaml:"resource_id"`
	CurrentPrice  int64   `yaml:"current_price"`
	PriceHistory  []int64 `yaml:"price_history,omitempty"`
}

// PersistedState is the full state written to state.yml.
type PersistedState struct {
	World   PersistedWorld   `yaml:"world"`
	Players []PersistedPlayer `yaml:"players"`
	Orders  []PersistedOrder  `yaml:"orders,omitempty"`
	Loans   []PersistedLoan   `yaml:"loans,omitempty"`
	Markets []PersistedMarketResource `yaml:"markets,omitempty"`
}

// Snapshot returns a serializable copy of the store state.
func (s *Store) Snapshot() *PersistedState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.world.mu.RLock()
	cycleID := s.world.CycleID
	currentDay := s.world.CurrentDay
	nextTickAt := s.world.NextTickAt
	s.world.mu.RUnlock()
	players := make([]PersistedPlayer, 0, len(s.players))
	for _, p := range s.players {
		p.mu.RLock()
		buildings := make([]PersistedBuilding, len(p.Buildings))
		for i := range p.Buildings {
			b := &p.Buildings[i]
			buildings[i] = PersistedBuilding{
				CellX:              b.CellX,
				CellY:              b.CellY,
				TypeID:             b.TypeID,
				Level:              b.Level,
				UpgradeTargetLevel: b.UpgradeTargetLevel,
				UpgradeFinishesAt:  b.UpgradeFinishesAt,
				SellingFinishesAt:  b.SellingFinishesAt,
			}
		}
		resCopy := make(map[string]int64)
		for k, v := range p.Resources {
			resCopy[k] = v
		}
		players = append(players, PersistedPlayer{
			ID:        p.ID,
			Coins:     p.Coins,
			Resources: resCopy,
			Buildings: buildings,
		})
		p.mu.RUnlock()
	}
	orders := make([]PersistedOrder, 0, len(s.orders))
	for _, o := range s.orders {
		orders = append(orders, PersistedOrder{
			OrderID:      o.OrderID,
			PlayerID:     o.PlayerID,
			OrderType:    o.OrderType,
			ResourceID:   o.ResourceID,
			Quantity:     o.Quantity,
			PricePerUnit: o.PricePerUnit,
			CreatedAt:    o.CreatedAt,
		})
	}
	loans := make([]PersistedLoan, 0, len(s.loans))
	for _, l := range s.loans {
		if l != nil {
			loans = append(loans, PersistedLoan{
				LoanID:         l.LoanID,
				PlayerID:       l.PlayerID,
				Balance:        l.Balance,
				InterestRate:   l.InterestRate,
				CreatedAt:      l.CreatedAt,
				OriginalAmount: l.OriginalAmount,
			})
		}
	}
	markets := make([]PersistedMarketResource, 0, len(s.market))
	for id, m := range s.market {
		if m != nil {
			hist := make([]int64, len(m.History))
			copy(hist, m.History)
			markets = append(markets, PersistedMarketResource{
				ResourceID:   id,
				CurrentPrice: m.CurrentPrice,
				PriceHistory: hist,
			})
		}
	}
	return &PersistedState{
		World:   PersistedWorld{CycleID: cycleID, CurrentDay: currentDay, NextTickAt: nextTickAt},
		Players: players,
		Orders:  orders,
		Loans:   loans,
		Markets: markets,
	}
}

// Restore replaces store state with the persisted state.
func (s *Store) Restore(ps *PersistedState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.world.mu.Lock()
	s.world.CycleID = ps.World.CycleID
	s.world.CurrentDay = ps.World.CurrentDay
	s.world.NextTickAt = ps.World.NextTickAt
	s.world.mu.Unlock()
	s.players = make(map[string]*Player)
	for _, pp := range ps.Players {
		buildings := make([]BuildingRow, len(pp.Buildings))
		for i := range pp.Buildings {
			pb := &pp.Buildings[i]
			typeID := pb.TypeID
			if typeID == "" && pb.Type != 0 {
				if pb.Type == 1 {
					typeID = "wheat_farm"
				} else if pb.Type == 2 {
					typeID = "stone_mine"
				}
			}
			buildings[i] = BuildingRow{
				CellX:              pb.CellX,
				CellY:              pb.CellY,
				TypeID:             typeID,
				Level:              pb.Level,
				UpgradeTargetLevel: pb.UpgradeTargetLevel,
				UpgradeFinishesAt:  pb.UpgradeFinishesAt,
				SellingFinishesAt:  pb.SellingFinishesAt,
			}
		}
		res := make(map[string]int64)
		if len(pp.Resources) > 0 {
			for k, v := range pp.Resources {
				res[k] = v
			}
		} else {
			if pp.Wheat != 0 {
				res["wheat"] = pp.Wheat
			}
			if pp.Stone != 0 {
				res["stone"] = pp.Stone
			}
		}
		s.players[pp.ID] = &Player{
			ID:        pp.ID,
			Coins:     pp.Coins,
			Resources: res,
			Buildings: buildings,
		}
	}
	s.orders = make(map[string]*Order)
	for _, po := range ps.Orders {
		s.orders[po.OrderID] = &Order{
			OrderID:      po.OrderID,
			PlayerID:     po.PlayerID,
			OrderType:    po.OrderType,
			ResourceID:   po.ResourceID,
			Quantity:     po.Quantity,
			PricePerUnit: po.PricePerUnit,
			CreatedAt:    po.CreatedAt,
		}
	}
	s.loans = make(map[string]*Loan)
	for _, pl := range ps.Loans {
		orig := pl.OriginalAmount
		if orig == 0 {
			orig = pl.Balance
		}
		s.loans[pl.LoanID] = &Loan{
			LoanID:         pl.LoanID,
			PlayerID:       pl.PlayerID,
			Balance:        pl.Balance,
			InterestRate:   pl.InterestRate,
			CreatedAt:      pl.CreatedAt,
			OriginalAmount: orig,
		}
	}
	s.market = make(map[string]*ResourceMarket)
	for _, pm := range ps.Markets {
		m := &ResourceMarket{
			CurrentPrice: pm.CurrentPrice,
			History:      append([]int64(nil), pm.PriceHistory...),
		}
		if len(m.History) == 0 {
			m.History = []int64{pm.CurrentPrice}
		}
		s.market[pm.ResourceID] = m
	}
}

// SaveToFile writes the current state to path (e.g. state.yml).
func (s *Store) SaveToFile(path string) error {
	snap := s.Snapshot()
	data, err := yaml.Marshal(snap)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadFromFile reads persisted state from path. Returns nil if file does not exist.
func LoadFromFile(path string) (*PersistedState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ps PersistedState
	if err := yaml.Unmarshal(data, &ps); err != nil {
		return nil, err
	}
	return &ps, nil
}

// RunCatchUpTicks runs RunTick for every tick that should have occurred between
// the world's NextTickAt and now, so that after a restart players receive
// resources for the missed period.
func (s *Store) RunCatchUpTicks(now time.Time, cfg *config.Config) int {
	_, _, nextTickAt := s.World().State()
	n := 0
	for !nextTickAt.After(now) {
		s.RunTick(nextTickAt, cfg)
		nextTickAt = nextTickAt.Add(cfg.TickInterval)
		n++
	}
	return n
}
