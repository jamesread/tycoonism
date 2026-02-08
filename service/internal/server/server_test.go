package server

import (
	"context"
	"testing"
	"time"

	"github.com/jamesread/httpauthshim/authpublic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"connectrpc.com/connect"
	gamev1 "webfarm/gen/game/v1"
	"webfarm/internal/config"
	"webfarm/internal/gamedata"
	"webfarm/internal/store"
)

func testServerBuildingDefs() []gamedata.BuildingDef {
	return []gamedata.BuildingDef{
		{ID: "wheat_farm", TickResources: map[string]int64{"wheat": 2}, UpgradeProductionFactor: map[string]float64{"wheat": 1.0}},
		{ID: "stone_mine", TickResources: map[string]int64{"stone": 3}, UpgradeProductionFactor: map[string]float64{"stone": 1.0}},
	}
}

func ctxWithPlayerID(id string) context.Context {
	return context.WithValue(context.Background(), PlayerIDHeader, id)
}

func TestWorldServer_Init_ReturnsEmptyAuthProvidersAndNoUsername(t *testing.T) {
	st := store.NewStore()
	srv := NewWorldServer(st, nil, nil)

	resp, err := srv.Init(context.Background(), connect.NewRequest(&gamev1.InitRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Msg.AuthenticationProviders)
	assert.Len(t, resp.Msg.AuthenticationProviders, 0)
	assert.Empty(t, resp.Msg.Username)
}

func TestWorldServer_Init_ReturnsUsernameWhenAuthenticated(t *testing.T) {
	st := store.NewStore()
	cfg := &config.Config{}
	srv := NewWorldServer(st, cfg, nil)
	ctx := ContextWithAuthUser(context.Background(), &authpublic.AuthenticatedUser{Username: "alice"})

	resp, err := srv.Init(ctx, connect.NewRequest(&gamev1.InitRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "alice", resp.Msg.Username)
}

func TestWorldServer_Init_ReturnsLocalLoginEnabledWhenLocalUsersEnabled(t *testing.T) {
	st := store.NewStore()
	cfg := &config.Config{
		Auth: &authpublic.Config{LocalUsers: authpublic.LocalUsersConfig{Enabled: true, Users: []*authpublic.LocalUser{{Username: "u", Password: "p"}}}},
	}
	srv := NewWorldServer(st, cfg, nil)

	resp, err := srv.Init(context.Background(), connect.NewRequest(&gamev1.InitRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Msg.LocalLoginEnabled)
}

func TestGameServer_AngelInvestor_Eligible(t *testing.T) {
	cfg := &config.Config{AngelInvestorCoins: 100}
	st := store.NewStore()
	st.GetOrCreatePlayer("p1", 0, map[string]int64{})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	resp, err := srv.AngelInvestor(ctx, connect.NewRequest(&gamev1.AngelInvestorRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(100), resp.Msg.State.Coins)
}

func TestGameServer_AngelInvestor_NotEligible_HasCoins(t *testing.T) {
	cfg := &config.Config{AngelInvestorCoins: 100}
	st := store.NewStore()
	st.GetOrCreatePlayer("p1", 5, map[string]int64{})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	_, err := srv.AngelInvestor(ctx, connect.NewRequest(&gamev1.AngelInvestorRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestGameServer_AngelInvestor_NotEligible_HasBuildings(t *testing.T) {
	cfg := &config.Config{AngelInvestorCoins: 100}
	st := store.NewStore()
	p := st.GetOrCreatePlayer("p1", 100, map[string]int64{})
	p.SpendCoins(100)
	p.AddBuilding(store.BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 1})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	_, err := srv.AngelInvestor(ctx, connect.NewRequest(&gamev1.AngelInvestorRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestGameServer_GetPlayerState_NewPlayerStartsWithWheatAndStone(t *testing.T) {
	cfg := &config.Config{
		StartingCoins:     100,
		StartingResources: map[string]int64{"wheat": 50, "stone": 50},
	}
	st := store.NewStore()
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("new-player")

	resp, err := srv.GetPlayerState(ctx, connect.NewRequest(&gamev1.GetPlayerStateRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(100), resp.Msg.Coins)
	assert.Equal(t, int64(50), resp.Msg.Resources["wheat"])
	assert.Equal(t, int64(50), resp.Msg.Resources["stone"])
}

func TestGameServer_GetPlayerState_ResourceRatesPerMinute(t *testing.T) {
	cfg := &config.Config{
		TickInterval: time.Minute,
		Buildings:   testServerBuildingDefs(),
	}
	st := store.NewStore()
	p := st.GetOrCreatePlayer("p1", 100, map[string]int64{})
	p.AddBuilding(store.BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 1})
	p.AddBuilding(store.BuildingRow{CellX: 1, CellY: 0, TypeID: "wheat_farm", Level: 3})
	p.AddBuilding(store.BuildingRow{CellX: 2, CellY: 0, TypeID: "stone_mine", Level: 2})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	resp, err := srv.GetPlayerState(ctx, connect.NewRequest(&gamev1.GetPlayerStateRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(8), resp.Msg.ResourcesPerMin["wheat"], "1*2 + 3*2 wheat/min")
	assert.Equal(t, int64(6), resp.Msg.ResourcesPerMin["stone"], "2*3 stone/min")
}

func TestGameServer_StartSellBuilding_Level1_ImmediateRefund(t *testing.T) {
	cfg := &config.Config{
		PlacementCost:    10,
		TickInterval:     time.Minute,
		Buildings:       testServerBuildingDefs(),
		UpgradeBaseDur:  time.Minute,
		UpgradeTimeStep: time.Minute,
		UpgradeMaxLevel: 10,
	}
	st := store.NewStore()
	p := st.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(store.BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 1})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	resp, err := srv.StartSellBuilding(ctx, connect.NewRequest(&gamev1.StartSellBuildingRequest{CellX: 0, CellY: 0}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.Msg.State.Grid, 0)
	assert.Equal(t, int64(5), resp.Msg.State.Coins, "refund 50% of placement cost for level 1")
}

func TestGameServer_StartSellBuilding_Level2_StartsSell(t *testing.T) {
	cfg := &config.Config{
		PlacementCost:    10,
		TickInterval:     time.Minute,
		Buildings:       testServerBuildingDefs(),
		UpgradeBaseDur:  time.Minute,
		UpgradeTimeStep: time.Minute,
		UpgradeMaxLevel: 10,
	}
	st := store.NewStore()
	p := st.GetOrCreatePlayer("p1", 0, map[string]int64{})
	p.AddBuilding(store.BuildingRow{CellX: 0, CellY: 0, TypeID: "wheat_farm", Level: 2})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	resp, err := srv.StartSellBuilding(ctx, connect.NewRequest(&gamev1.StartSellBuildingRequest{CellX: 0, CellY: 0}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Msg.State.Grid, 1)
	assert.Greater(t, resp.Msg.State.Grid[0].SellingFinishesAt, int64(0))
}

func TestGameServer_GetLeaderboard_SortedByCoinsDesc(t *testing.T) {
	st := store.NewStore()
	st.GetOrCreatePlayer("alice", 50, map[string]int64{})
	st.GetOrCreatePlayer("bob", 200, map[string]int64{})
	st.GetOrCreatePlayer("carol", 100, map[string]int64{})
	srv := NewGameServer(st, nil, nil)

	resp, err := srv.GetLeaderboard(context.Background(), connect.NewRequest(&gamev1.GetLeaderboardRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Msg.Entries, 3)
	assert.Equal(t, "bob", resp.Msg.Entries[0].PlayerId)
	assert.Equal(t, int64(200), resp.Msg.Entries[0].Coins)
	assert.Equal(t, "carol", resp.Msg.Entries[1].PlayerId)
	assert.Equal(t, int64(100), resp.Msg.Entries[1].Coins)
	assert.Equal(t, "alice", resp.Msg.Entries[2].PlayerId)
	assert.Equal(t, int64(50), resp.Msg.Entries[2].Coins)
}

func TestGameServer_GetLeaderboard_ExcludesNPCs(t *testing.T) {
	st := store.NewStore()
	st.GetOrCreatePlayer("alice", 100, map[string]int64{})
	st.GetOrCreateNPCPlayer("Merchant", 1e9)
	st.GetOrCreatePlayer("bob", 50, map[string]int64{})
	srv := NewGameServer(st, nil, nil)

	resp, err := srv.GetLeaderboard(context.Background(), connect.NewRequest(&gamev1.GetLeaderboardRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Msg.Entries, 2)
	assert.Equal(t, "alice", resp.Msg.Entries[0].PlayerId)
	assert.Equal(t, "bob", resp.Msg.Entries[1].PlayerId)
}

func TestRunNPCTick_NoNPCs_NoPanic(t *testing.T) {
	st := store.NewStore()
	cfg := &config.Config{NPCs: nil, Resources: []gamedata.ResourceDef{{ID: "wheat"}}}
	RunNPCTick(st, cfg)
	sell := st.GetSellOrders("")
	buy := st.GetBuyOrders("")
	assert.Len(t, sell, 0)
	assert.Len(t, buy, 0)
}

func TestRunNPCTick_WithNPCs_PlacesOrdersEventually(t *testing.T) {
	st := store.NewStore()
	cfg := &config.Config{
		Resources: []gamedata.ResourceDef{{ID: "wheat"}},
		NPCs: []gamedata.NPCDef{{
			Name:   "TestNPC",
			Wealth: 50,
			Risk:   50,
			Resources: []gamedata.NPCResource{{
				ResourceType:   "wheat",
				BuyEagerness:   100,
				SellEagerness:  100,
				MaxCapacity:    10,
			}},
		}},
	}
	for i := 0; i < 300; i++ {
		RunNPCTick(st, cfg)
	}
	sell := st.GetSellOrders("")
	buy := st.GetBuyOrders("")
	total := len(sell) + len(buy)
	assert.Greater(t, total, 0, "RunNPCTick with 100 eagerness should place at least one order over 300 ticks")
}

func TestGameServer_PlaceSellOrder_RejectsPriceBelowOne(t *testing.T) {
	cfg := &config.Config{Resources: []gamedata.ResourceDef{{ID: "wheat"}}}
	st := store.NewStore()
	st.GetOrCreatePlayer("p1", 0, map[string]int64{"wheat": 10})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	_, err := srv.PlaceSellOrder(ctx, connect.NewRequest(&gamev1.PlaceSellOrderRequest{
		ResourceId:   "wheat",
		Quantity:     1,
		PricePerUnit: 0,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestGameServer_PlaceBuyOrder_RejectsPriceBelowOne(t *testing.T) {
	cfg := &config.Config{Resources: []gamedata.ResourceDef{{ID: "wheat"}}}
	st := store.NewStore()
	st.GetOrCreatePlayer("p1", 100, map[string]int64{})
	srv := NewGameServer(st, cfg, nil)
	ctx := ctxWithPlayerID("p1")

	_, err := srv.PlaceBuyOrder(ctx, connect.NewRequest(&gamev1.PlaceBuyOrderRequest{
		ResourceId:   "wheat",
		Quantity:     1,
		PricePerUnit: 0,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
