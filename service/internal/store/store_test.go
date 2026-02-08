package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"webfarm/internal/config"
	"webfarm/internal/gamedata"
)

func testBuildingDefs() []gamedata.BuildingDef {
	return []gamedata.BuildingDef{
		{ID: "wheat_farm", TickResources: map[string]int64{"wheat": 1}, UpgradeProductionFactor: map[string]float64{"wheat": 1.0}},
		{ID: "stone_mine", TickResources: map[string]int64{"stone": 1}, UpgradeProductionFactor: map[string]float64{"stone": 1.0}},
	}
}

func TestStore_GetOrCreatePlayer(t *testing.T) {
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 100, map[string]int64{})
	require.NotNil(t, p)
	assert.Equal(t, "p1", p.ID)
	assert.Equal(t, int64(100), p.GetCoins())
	assert.Equal(t, int64(0), p.GetResource("wheat"))
	assert.Equal(t, int64(0), p.GetResource("stone"))
	p2 := s.GetOrCreatePlayer("p1", 0, nil)
	assert.Same(t, p, p2)

	p3 := s.GetOrCreatePlayer("p2", 50, map[string]int64{"wheat": 50, "stone": 50})
	require.NotNil(t, p3)
	assert.Equal(t, int64(50), p3.GetCoins())
	assert.Equal(t, int64(50), p3.GetResource("wheat"))
	assert.Equal(t, int64(50), p3.GetResource("stone"))
}

func TestNPCPlayerID(t *testing.T) {
	assert.Equal(t, "npc:Alice", NPCPlayerID("Alice"))
	assert.Equal(t, "npc:merchant_1", NPCPlayerID("merchant_1"))
}

func TestIsNPCPlayerID(t *testing.T) {
	assert.True(t, IsNPCPlayerID("npc:Alice"))
	assert.True(t, IsNPCPlayerID("npc:x"))
	assert.False(t, IsNPCPlayerID("alice"))
	assert.False(t, IsNPCPlayerID(""))
	assert.False(t, IsNPCPlayerID("npc")) // prefix is "npc:", so "npc" is not an NPC ID
}

func TestStore_GetOrCreateNPCPlayer(t *testing.T) {
	s := NewStore()
	p := s.GetOrCreateNPCPlayer("Trader", 1e9)
	require.NotNil(t, p)
	assert.Equal(t, "npc:Trader", p.ID)
	assert.Equal(t, int64(1e9), p.GetCoins())
	assert.Equal(t, int64(0), p.GetResource("wheat"))
	p2 := s.GetOrCreateNPCPlayer("Trader", 0)
	assert.Same(t, p, p2)
	assert.Equal(t, int64(1e9), p2.GetCoins())
}

func TestPlayer_SpendCoins(t *testing.T) {
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 50, map[string]int64{})
	assert.False(t, p.SpendCoins(100))
	assert.True(t, p.SpendCoins(30))
	assert.Equal(t, int64(20), p.GetCoins())
	assert.False(t, p.SpendCoins(-1))
}

func TestPlayer_PlaceAndBuildingAt(t *testing.T) {
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 100, map[string]int64{})
	assert.False(t, p.HasBuildingAt(0, 0))
	p.AddBuilding(BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 1})
	assert.True(t, p.HasBuildingAt(0, 0))
	b := p.BuildingAt(0, 0)
	require.NotNil(t, b)
	assert.Equal(t, int32(1), b.Level)
}

func TestPlayer_RemoveBuilding(t *testing.T) {
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 1})
	p.AddBuilding(BuildingRow{CellX: 1, CellY: 1, TypeID: "stone_mine", Level: 1})
	assert.True(t, p.RemoveBuilding(0, 0))
	assert.False(t, p.HasBuildingAt(0, 0))
	assert.True(t, p.HasBuildingAt(1, 1))
	assert.False(t, p.RemoveBuilding(0, 0))
	assert.True(t, p.RemoveBuilding(1, 1))
	assert.False(t, p.HasBuildingAt(1, 1))
}

func TestPlayer_StartSell(t *testing.T) {
	finishesAt := time.Now().Add(time.Minute)
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 2})
	assert.True(t, p.StartSell(0, 0, finishesAt))
	b := p.BuildingAt(0, 0)
	require.NotNil(t, b)
	assert.True(t, b.SellingFinishesAt.Equal(finishesAt))
	assert.False(t, p.StartSell(1, 1, time.Now()))
}

func TestStore_RunTick_CompletesSellAndRefunds(t *testing.T) {
	cfg := &config.Config{
		PlacementCost:  10,
		DaysPerCycle:  7,
		TickInterval:  time.Minute,
		Buildings:     testBuildingDefs(),
		UpgradeBaseDur: time.Minute,
		UpgradeTimeStep: time.Minute,
		UpgradeMaxLevel: 10,
	}
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(BuildingRow{
		CellX: 0, CellY: 0,
		TypeID: "wheat_farm",
		Level:  2,
		SellingFinishesAt: time.Now().Add(-time.Second),
	})
	s.RunTick(time.Now(), cfg)
	assert.False(t, p.HasBuildingAt(0, 0))
	assert.Equal(t, int64(10), p.GetCoins(), "refund = placement_cost * level / 2 = 10*2/2")
}

func TestStore_RunTick_CompletesUpgradesAndAddsResources(t *testing.T) {
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(BuildingRow{
		CellX: 0, CellY: 0,
		TypeID: "wheat_farm",
		Level:  1,
	})
	p.AddBuilding(BuildingRow{
		CellX: 1, CellY: 1,
		TypeID: "stone_mine",
		Level:  2,
	})
	cfg := &config.Config{
		DaysPerCycle: 7,
		TickInterval: time.Minute,
		Buildings:    testBuildingDefs(),
	}
	now := time.Now()
	s.RunTick(now, cfg)
	assert.Equal(t, int64(1), p.GetResource("wheat"))
	assert.Equal(t, int64(2), p.GetResource("stone"))
	cycleID, day, _ := s.World().State()
	assert.Equal(t, int64(0), cycleID)
	assert.Equal(t, int32(2), day)
}

func TestStore_SnapshotRestore_RoundTrip(t *testing.T) {
	cfg := &config.Config{DaysPerCycle: 7, TickInterval: time.Minute, Buildings: testBuildingDefs()}
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 50, map[string]int64{})
	p.AddBuilding(BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 2})
	s.RunTick(time.Now(), cfg)
	beforeCycle, beforeDay, beforeNext := s.World().State()
	assert.Equal(t, int64(2), p.GetResource("wheat"), "level 2 farm produces 2 wheat per tick")

	snap := s.Snapshot()
	s2 := NewStore()
	s2.Restore(snap)
	afterCycle, afterDay, afterNext := s2.World().State()
	assert.Equal(t, beforeCycle, afterCycle)
	assert.Equal(t, beforeDay, afterDay)
	assert.True(t, beforeNext.Equal(afterNext))
	p2 := s2.GetPlayer("p1")
	require.NotNil(t, p2)
	assert.Equal(t, p.GetCoins(), p2.GetCoins())
	assert.Equal(t, p.GetResource("wheat"), p2.GetResource("wheat"))
	assert.Equal(t, p.GetResource("stone"), p2.GetResource("stone"))
}

func TestStore_SaveToFile_LoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yml")
	s := NewStore()
	s.GetOrCreatePlayer("alice", 100, map[string]int64{})
	err := s.SaveToFile(path)
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.NoError(t, err)

	ps, err := LoadFromFile(path)
	require.NoError(t, err)
	require.NotNil(t, ps)
	assert.Len(t, ps.Players, 1)
	assert.Equal(t, "alice", ps.Players[0].ID)
	assert.Equal(t, int64(100), ps.Players[0].Coins)

	s2 := NewStore()
	s2.Restore(ps)
	p := s2.GetPlayer("alice")
	require.NotNil(t, p)
	assert.Equal(t, int64(100), p.GetCoins())
}

func TestStore_LoadFromFile_NoFile(t *testing.T) {
	ps, err := LoadFromFile(filepath.Join(t.TempDir(), "nonexistent.yml"))
	require.NoError(t, err)
	assert.Nil(t, ps)
}

func TestStore_RunCatchUpTicks_GeneratesMissedResources(t *testing.T) {
	buildings := []gamedata.BuildingDef{
		{ID: "wheat_farm", TickResources: map[string]int64{"wheat": 2}, UpgradeProductionFactor: map[string]float64{"wheat": 1.0}},
		{ID: "stone_mine", TickResources: map[string]int64{"stone": 1}, UpgradeProductionFactor: map[string]float64{"stone": 1.0}},
	}
	cfg := &config.Config{
		DaysPerCycle: 7,
		TickInterval: time.Minute,
		Buildings:    buildings,
	}
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 1})
	p.AddBuilding(BuildingRow{CellX: 1, CellY: 0, TypeID: "stone_mine", Level: 1})

	base := time.Now().Add(-20 * time.Minute)
	s.world.mu.Lock()
	s.world.NextTickAt = base
	s.world.mu.Unlock()

	n := s.RunCatchUpTicks(time.Now(), cfg)
	assert.GreaterOrEqual(t, n, 1)
	assert.Equal(t, int64(n*2), p.GetResource("wheat"), "wheat: 2 per tick")
	assert.Equal(t, int64(n), p.GetResource("stone"), "stone: 1 per tick")
}

func TestStore_GetMarketPrice_DefaultWhenNoMarket(t *testing.T) {
	s := NewStore()
	assert.Equal(t, int64(10), s.GetMarketPrice("wheat"))
	avgs := s.GetMarketPriceAverages("wheat")
	assert.Equal(t, int64(10), avgs.CurrentPrice)
	assert.Equal(t, 0.0, avgs.AvgChange3Tick)
}

func TestStore_RunTick_AppliesLoanInterest(t *testing.T) {
	cfg := &config.Config{
		DaysPerCycle:  7,
		TickInterval:  time.Minute,
		Resources:    []gamedata.ResourceDef{{ID: "wheat"}},
		Buildings:    testBuildingDefs(),
	}
	s := NewStore()
	p := s.GetOrCreatePlayer("p1", 0, map[string]int64{})
	s.AddLoan(&Loan{
		LoanID:         "loan-1",
		PlayerID:       "p1",
		Balance:        10_000,
		InterestRate:   0.01,
		CreatedAt:      time.Now(),
		OriginalAmount: 10_000,
	})
	p.SpendCoins(10_000)
	assert.Equal(t, int64(0), p.GetCoins(), "player spent all loan so has 0 coins")
	loans := s.GetLoansByPlayer("p1")
	require.Len(t, loans, 1)
	assert.Equal(t, int64(10_000), loans[0].Balance)

	s.RunTick(time.Now(), cfg)

	loans = s.GetLoansByPlayer("p1")
	require.Len(t, loans, 1)
	interestAdded := int64(float64(10_000) * 0.01)
	assert.Equal(t, int64(10_000+interestAdded), loans[0].Balance,
		"interest (1%% of 10000 = %d) should be added to loan balance when player has no coins", interestAdded)
	assert.Equal(t, int64(0), p.GetCoins())
}

func TestStore_RunTick_InitializesMarketPrice(t *testing.T) {
	cfg := &config.Config{
		DaysPerCycle:  7,
		TickInterval:  time.Minute,
		Resources:     []gamedata.ResourceDef{{ID: "wheat"}},
		Buildings:     testBuildingDefs(),
	}
	s := NewStore()
	s.RunTick(time.Now(), cfg)
	assert.Equal(t, int64(10), s.GetMarketPrice("wheat"))
}

func TestStore_MarketPrice_RespondsToSupplyDemand(t *testing.T) {
	cfg := &config.Config{
		DaysPerCycle:  7,
		TickInterval:  time.Minute,
		Resources:     []gamedata.ResourceDef{{ID: "wheat"}},
		Buildings:     testBuildingDefs(),
	}
	s := NewStore()
	s.RunTick(time.Now(), cfg)
	priceNoOrders := s.GetMarketPrice("wheat")
	s.AddOrder(&Order{OrderID: "b1", OrderType: OrderTypeBuy, ResourceID: "wheat", Quantity: 500, PricePerUnit: 10})
	s.RunTick(time.Now().Add(time.Minute), cfg)
	priceAfterBuy := s.GetMarketPrice("wheat")
	assert.Greater(t, priceAfterBuy, priceNoOrders, "demand from buy orders should push price up")
	seller := s.GetOrCreatePlayer("seller", 0, map[string]int64{"wheat": 3000})
	seller.SellResource("wheat", 2000)
	s.AddOrder(&Order{OrderID: "s1", PlayerID: seller.ID, OrderType: OrderTypeSell, ResourceID: "wheat", Quantity: 2000, PricePerUnit: 5})
	s.RunTick(time.Now().Add(2*time.Minute), cfg)
	priceAfterSell := s.GetMarketPrice("wheat")
	assert.Less(t, priceAfterSell, priceAfterBuy, "excess supply from sell orders should push price down")
}

func TestStore_SnapshotRestore_IncludesMarkets(t *testing.T) {
	cfg := &config.Config{
		DaysPerCycle:  7,
		TickInterval:  time.Minute,
		Resources:     []gamedata.ResourceDef{{ID: "wheat"}, {ID: "stone"}},
		Buildings:     testBuildingDefs(),
	}
	s := NewStore()
	s.RunTick(time.Now(), cfg)
	s.RunTick(time.Now().Add(time.Minute), cfg)
	wheatPrice := s.GetMarketPrice("wheat")
	ps := s.Snapshot()
	require.NotEmpty(t, ps.Markets)
	s2 := NewStore()
	s2.Restore(ps)
	assert.Equal(t, wheatPrice, s2.GetMarketPrice("wheat"))
	assert.Equal(t, s.GetMarketPrice("stone"), s2.GetMarketPrice("stone"))
}
