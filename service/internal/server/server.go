package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	mrand "math/rand"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jamesread/httpauthshim/authpublic"
	"github.com/jamesread/httpauthshim/providers/haslocal"
	auth "github.com/jamesread/httpauthshim"
	"github.com/sirupsen/logrus"
	"connectrpc.com/connect"
	gamev1 "webfarm/gen/game/v1"
	"webfarm/gen/game/v1/gamev1connect"
	"webfarm/internal/config"
	"webfarm/internal/game"
	"webfarm/internal/gamedata"
	"webfarm/internal/store"
)

const PlayerIDHeader = "X-Player-ID"

type authUserKeyType struct{}

var authUserKey = &authUserKeyType{}

// AuthChecker runs the auth chain on a request and returns the authenticated user (or guest).
type AuthChecker interface {
	AuthFromHttpReq(r *http.Request) *authpublic.AuthenticatedUser
}

// AuthUserFromContext returns the authenticated user from context, or nil if not set.
func AuthUserFromContext(ctx context.Context) *authpublic.AuthenticatedUser {
	u, _ := ctx.Value(authUserKey).(*authpublic.AuthenticatedUser)
	return u
}

// ContextWithAuthUser returns a context with the authenticated user set (for tests or middleware).
func ContextWithAuthUser(ctx context.Context, user *authpublic.AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authUserKey, user)
}

type WorldServer struct {
	store *store.Store
	cfg   *config.Config
	log   *logrus.Logger
}

func NewWorldServer(store *store.Store, cfg *config.Config, log *logrus.Logger) *WorldServer {
	return &WorldServer{store: store, cfg: cfg, log: log}
}

func resourceDefsToProto(defs []gamedata.ResourceDef, str *store.Store) []*gamev1.ResourceDef {
	out := make([]*gamev1.ResourceDef, 0, len(defs))
	for i := range defs {
		d := &defs[i]
		price := int64(0)
		var avg3, avg10, avg50 float64
		if str != nil {
			price = str.GetMarketPrice(d.ID)
			avgs := str.GetMarketPriceAverages(d.ID)
			avg3, avg10, avg50 = avgs.AvgChange3Tick, avgs.AvgChange10Tick, avgs.AvgChange50Tick
		}
		out = append(out, &gamev1.ResourceDef{
			Id:                   d.ID,
			Name:                 d.Name,
			Icon:                 d.Icon,
			SellPrice:            price,
			BuyPrice:             price,
			BaseColor:            d.BaseColor,
			PriceChangeAvg_3Tick: avg3,
			PriceChangeAvg_10Tick: avg10,
			PriceChangeAvg_50Tick: avg50,
		})
	}
	return out
}

func buildingDefsToProto(defs []gamedata.BuildingDef, cfg *config.Config) []*gamev1.BuildingDef {
	out := make([]*gamev1.BuildingDef, 0, len(defs))
	for i := range defs {
		d := &defs[i]
		var reqs []*gamev1.BuildingRequirement
		for j := range d.Requirements {
			r := &d.Requirements[j]
			reqs = append(reqs, &gamev1.BuildingRequirement{
				BuildingId: r.BuildingID,
				Count:      int32(r.Count),
			})
		}
		placementCost := int64(0)
		if cfg != nil {
			placementCost = cfg.PlacementCostForBuilding(d)
		}
		out = append(out, &gamev1.BuildingDef{
			Id:                      d.ID,
			Name:                    d.Name,
			Icon:                    d.Icon,
			BaseLevel:               d.BaseLevel,
			TickResources:           d.TickResources,
			UpgradeProductionFactor: d.UpgradeProductionFactor,
			UpgradeTimeFactor:       d.UpgradeTimeFactor,
			Requirements:            reqs,
			PlacementCost:           placementCost,
		})
	}
	return out
}

func (s *WorldServer) Init(ctx context.Context, req *connect.Request[gamev1.InitRequest]) (*connect.Response[gamev1.InitResponse], error) {
	resp := &gamev1.InitResponse{AuthenticationProviders: []*gamev1.AuthProvider{}}
	if s.cfg != nil && s.cfg.Auth != nil {
		if s.cfg.Auth.LocalUsers.Enabled {
			resp.LocalLoginEnabled = true
		}
		if len(s.cfg.Auth.OAuth2Providers) > 0 {
			for id, p := range s.cfg.Auth.OAuth2Providers {
				name := id
				if p != nil && p.Title != "" {
					name = p.Title
				}
				resp.AuthenticationProviders = append(resp.AuthenticationProviders, &gamev1.AuthProvider{Id: id, Name: name})
			}
			sort.Slice(resp.AuthenticationProviders, func(i, j int) bool {
				return resp.AuthenticationProviders[i].Id < resp.AuthenticationProviders[j].Id
			})
		}
	}
	if user := AuthUserFromContext(ctx); user != nil && !user.IsGuest() {
		resp.Username = user.Username
	}
	return connect.NewResponse(resp), nil
}

func worldEndsAtUnix(cfg *config.Config) int64 {
	if cfg == nil || cfg.WorldEndsAt.IsZero() {
		return 0
	}
	return cfg.WorldEndsAt.Unix()
}

func (s *WorldServer) GetWorldState(ctx context.Context, req *connect.Request[gamev1.GetWorldStateRequest]) (*connect.Response[gamev1.WorldState], error) {
	world := s.store.World()
	cycleID, currentDay, nextTickAt := world.State()
	tickSecs := int64(s.cfg.TickInterval / time.Second)
	if tickSecs < 1 {
		tickSecs = 1
	}
	return connect.NewResponse(&gamev1.WorldState{
		CurrentDay:            currentDay,
		CycleId:               cycleID,
		NextTickAt:            nextTickAt.Unix(),
		TickIntervalSeconds:   tickSecs,
		PlacementCost:         s.cfg.PlacementCost,
		ResourceDefinitions:  resourceDefsToProto(s.cfg.Resources, s.store),
		BuildingDefinitions:  buildingDefsToProto(s.cfg.Buildings, s.cfg),
		WorldEndsAt:           worldEndsAtUnix(s.cfg),
		ServerTimeUnix:        time.Now().Unix(),
	}), nil
}

type GameServer struct {
	store *store.Store
	cfg   *config.Config
	log   *logrus.Logger
}

func NewGameServer(store *store.Store, cfg *config.Config, log *logrus.Logger) *GameServer {
	return &GameServer{store: store, cfg: cfg, log: log}
}

func playerIDFromContext(ctx context.Context) (string, error) {
	header := ctx.Value(PlayerIDHeader)
	if s, ok := header.(string); ok && s != "" {
		return s, nil
	}
	return "", connect.NewError(connect.CodeUnauthenticated, errors.New("missing X-Player-ID"))
}

func (s *GameServer) getOrCreatePlayer(ctx context.Context) (*store.Player, error) {
	id, err := playerIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.GetOrCreatePlayer(id, s.cfg.StartingCoins, s.cfg.StartingResources), nil
}

func playerStateFromStore(p *store.Player, cfg *config.Config, sellOrderIDs, buyOrderIDs []string) *gamev1.PlayerState {
	buildings := p.AllBuildings()
	grid := make([]*gamev1.Building, 0, len(buildings))
	perMin := make(map[string]int64)
	for _, b := range buildings {
		upgradeAt := int64(0)
		if !b.UpgradeFinishesAt.IsZero() {
			upgradeAt = b.UpgradeFinishesAt.Unix()
		}
		sellingAt := int64(0)
		if !b.SellingFinishesAt.IsZero() {
			sellingAt = b.SellingFinishesAt.Unix()
		}
		grid = append(grid, &gamev1.Building{
			CellX:             b.CellX,
			CellY:             b.CellY,
			TypeId:            b.TypeID,
			Level:             b.Level,
			UpgradeFinishesAt: upgradeAt,
			SellingFinishesAt: sellingAt,
		})
		produced := game.ResourceProducedByBuilding(b.TypeID, b.Level, cfg.Buildings)
		for resID, amount := range produced {
			perMin[resID] += amount * int64(time.Minute) / int64(cfg.TickInterval)
		}
	}
	return &gamev1.PlayerState{
		PlayerId:        p.ID,
		Coins:           p.GetCoins(),
		Resources:       p.CopyResources(),
		Grid:            grid,
		ResourcesPerMin: perMin,
		SellOrderIds:    sellOrderIDs,
		BuyOrderIds:     buyOrderIDs,
	}
}

func (s *GameServer) playerState(p *store.Player) *gamev1.PlayerState {
	sellIDs, buyIDs := s.store.OrdersByPlayer(p.ID)
	return playerStateFromStore(p, s.cfg, sellIDs, buyIDs)
}

func (s *GameServer) GetPlayerState(ctx context.Context, req *connect.Request[gamev1.GetPlayerStateRequest]) (*connect.Response[gamev1.PlayerState], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(s.playerState(p)), nil
}

func (s *GameServer) PlaceBuilding(ctx context.Context, req *connect.Request[gamev1.PlaceBuildingRequest]) (*connect.Response[gamev1.PlaceBuildingResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	typeID := req.Msg.BuildingTypeId
	def := gamedata.BuildingByID(s.cfg.Buildings, typeID)
	if def == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid building type"))
	}
	x, y := req.Msg.CellX, req.Msg.CellY
	if x < 0 || x >= store.GridSize || y < 0 || y >= store.GridSize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cell out of bounds"))
	}
	if p.HasBuildingAt(x, y) {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("cell occupied"))
	}
	for _, preq := range def.Requirements {
		count := 0
		for _, b := range p.AllBuildings() {
			if b.TypeID == preq.BuildingID {
				count++
			}
		}
		if count < preq.Count {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("building requirements not met"))
		}
	}
	cost := s.cfg.PlacementCostForBuilding(def)
	if !p.SpendCoins(cost) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient coins"))
	}
	level := def.BaseLevel
	if level < 1 {
		level = 1
	}
	p.AddBuilding(store.BuildingRow{
		CellX:  x,
		CellY:  y,
		TypeID: typeID,
		Level:  level,
	})
	return connect.NewResponse(&gamev1.PlaceBuildingResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) StartUpgrade(ctx context.Context, req *connect.Request[gamev1.StartUpgradeRequest]) (*connect.Response[gamev1.StartUpgradeResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	x, y := req.Msg.CellX, req.Msg.CellY
	if x < 0 || x >= store.GridSize || y < 0 || y >= store.GridSize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cell out of bounds"))
	}
	b := p.BuildingAt(x, y)
	if b == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no building at cell"))
	}
	if !b.UpgradeFinishesAt.IsZero() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("upgrade already in progress"))
	}
	nextLevel := b.Level + 1
	if nextLevel > int32(s.cfg.UpgradeMaxLevel) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("already max level"))
	}
	cost := game.UpgradeCost(s.cfg, nextLevel)
	if !p.SpendCoins(cost) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient coins"))
	}
	def := gamedata.BuildingByID(s.cfg.Buildings, b.TypeID)
	timeFactor := 1.0
	if def != nil {
		timeFactor = def.UpgradeTimeFactor
	}
	dur := game.UpgradeDuration(s.cfg, b.Level, timeFactor)
	finishesAt := time.Now().Add(dur)
	p.StartUpgrade(x, y, nextLevel, finishesAt)
	return connect.NewResponse(&gamev1.StartUpgradeResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) SellResource(ctx context.Context, req *connect.Request[gamev1.SellResourceRequest]) (*connect.Response[gamev1.SellResourceResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	resourceID := req.Msg.ResourceId
	if gamedata.ResourceByID(s.cfg.Resources, resourceID) == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid resource type"))
	}
	pricePerUnit := s.store.GetMarketPrice(resourceID)
	qty := req.Msg.Quantity
	sold := p.SellResource(resourceID, qty)
	earned := sold * pricePerUnit
	p.AddCoins(earned)
	return connect.NewResponse(&gamev1.SellResourceResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) BuyResource(ctx context.Context, req *connect.Request[gamev1.BuyResourceRequest]) (*connect.Response[gamev1.BuyResourceResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	resourceID := req.Msg.ResourceId
	if gamedata.ResourceByID(s.cfg.Resources, resourceID) == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid resource type"))
	}
	pricePerUnit := s.store.GetMarketPrice(resourceID)
	qty := req.Msg.Quantity
	if qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("quantity must be positive"))
	}
	cost := qty * pricePerUnit
	if !p.SpendCoins(cost) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient coins"))
	}
	p.AddResource(resourceID, qty)
	return connect.NewResponse(&gamev1.BuyResourceResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) CancelUpgrade(ctx context.Context, req *connect.Request[gamev1.CancelUpgradeRequest]) (*connect.Response[gamev1.CancelUpgradeResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	x, y := req.Msg.CellX, req.Msg.CellY
	if x < 0 || x >= store.GridSize || y < 0 || y >= store.GridSize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cell out of bounds"))
	}
	b := p.BuildingAt(x, y)
	if b == nil || b.UpgradeFinishesAt.IsZero() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no upgrade in progress"))
	}
	p.CancelUpgrade(x, y)
	return connect.NewResponse(&gamev1.CancelUpgradeResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) StartSellBuilding(ctx context.Context, req *connect.Request[gamev1.StartSellBuildingRequest]) (*connect.Response[gamev1.StartSellBuildingResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	x, y := req.Msg.CellX, req.Msg.CellY
	if x < 0 || x >= store.GridSize || y < 0 || y >= store.GridSize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cell out of bounds"))
	}
	b := p.BuildingAt(x, y)
	if b == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no building at cell"))
	}
	if !b.UpgradeFinishesAt.IsZero() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot sell while upgrading"))
	}
	if !b.SellingFinishesAt.IsZero() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("sell already in progress"))
	}
	def := gamedata.BuildingByID(s.cfg.Buildings, b.TypeID)
	timeFactor := 1.0
	if def != nil {
		timeFactor = def.UpgradeTimeFactor
	}
	dur := game.SellDuration(s.cfg, b.Level, timeFactor)
	if dur == 0 {
		refund := game.SellRefund(s.cfg, b.Level)
		p.AddCoins(refund)
		p.RemoveBuilding(x, y)
		return connect.NewResponse(&gamev1.StartSellBuildingResponse{
			State: s.playerState(p),
		}), nil
	}
	finishesAt := time.Now().Add(dur)
	if !p.StartSell(x, y, finishesAt) {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to start sell"))
	}
	return connect.NewResponse(&gamev1.StartSellBuildingResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) GetLeaderboard(ctx context.Context, req *connect.Request[gamev1.GetLeaderboardRequest]) (*connect.Response[gamev1.GetLeaderboardResponse], error) {
	players := s.store.AllPlayers()
	entries := make([]*gamev1.LeaderboardEntry, 0, len(players))
	for _, p := range players {
		if store.IsNPCPlayerID(p.ID) {
			continue
		}
		entries = append(entries, &gamev1.LeaderboardEntry{
			PlayerId: p.ID,
			Coins:    p.GetCoins(),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Coins > entries[j].Coins })
	return connect.NewResponse(&gamev1.GetLeaderboardResponse{Entries: entries}), nil
}

func (s *GameServer) AngelInvestor(ctx context.Context, req *connect.Request[gamev1.AngelInvestorRequest]) (*connect.Response[gamev1.AngelInvestorResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	if p.GetCoins() != 0 || len(p.AllBuildings()) != 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("angel investor only available with 0 coins and 0 buildings"))
	}
	p.AddCoins(s.cfg.AngelInvestorCoins)
	return connect.NewResponse(&gamev1.AngelInvestorResponse{
		State: s.playerState(p),
	}), nil
}

func marketOrderToProto(o *store.Order) *gamev1.MarketOrder {
	if o == nil {
		return nil
	}
	ot := gamev1.OrderType_ORDER_TYPE_UNSPECIFIED
	if o.OrderType == store.OrderTypeSell {
		ot = gamev1.OrderType_ORDER_TYPE_SELL
	} else if o.OrderType == store.OrderTypeBuy {
		ot = gamev1.OrderType_ORDER_TYPE_BUY
	}
	return &gamev1.MarketOrder{
		OrderId:      o.OrderID,
		PlayerId:     o.PlayerID,
		OrderType:    ot,
		ResourceId:   o.ResourceID,
		Quantity:     o.Quantity,
		PricePerUnit: o.PricePerUnit,
		CreatedAt:    o.CreatedAt.Unix(),
	}
}

func generateOrderID() string {
	return uuid.New().String()
}

func (s *GameServer) GetMarketplace(ctx context.Context, req *connect.Request[gamev1.GetMarketplaceRequest]) (*connect.Response[gamev1.GetMarketplaceResponse], error) {
	resourceID := req.Msg.ResourceId
	sellOrders := s.store.GetSellOrders(resourceID)
	buyOrders := s.store.GetBuyOrders(resourceID)
	sellProto := make([]*gamev1.MarketOrder, 0, len(sellOrders))
	for _, o := range sellOrders {
		sellProto = append(sellProto, marketOrderToProto(o))
	}
	buyProto := make([]*gamev1.MarketOrder, 0, len(buyOrders))
	for _, o := range buyOrders {
		buyProto = append(buyProto, marketOrderToProto(o))
	}
	return connect.NewResponse(&gamev1.GetMarketplaceResponse{
		SellOrders: sellProto,
		BuyOrders:  buyProto,
	}), nil
}

func (s *GameServer) GetResourcePriceHistory(ctx context.Context, req *connect.Request[gamev1.GetResourcePriceHistoryRequest]) (*connect.Response[gamev1.GetResourcePriceHistoryResponse], error) {
	resourceID := req.Msg.ResourceId
	if resourceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("resource_id required"))
	}
	ticks := req.Msg.Ticks
	if ticks <= 0 {
		ticks = 15
	}
	history := s.store.GetMarketPriceHistory(resourceID, int(ticks))
	return connect.NewResponse(&gamev1.GetResourcePriceHistoryResponse{
		PriceHistory: history,
	}), nil
}

func (s *GameServer) GetMessages(ctx context.Context, req *connect.Request[gamev1.GetMessagesRequest]) (*connect.Response[gamev1.GetMessagesResponse], error) {
	return connect.NewResponse(&gamev1.GetMessagesResponse{
		Messages: []*gamev1.GameMessage{},
	}), nil
}

func (s *GameServer) PlaceSellOrder(ctx context.Context, req *connect.Request[gamev1.PlaceSellOrderRequest]) (*connect.Response[gamev1.PlaceSellOrderResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	resourceID := req.Msg.ResourceId
	if resourceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("resource_id required"))
	}
	qty := req.Msg.Quantity
	if qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("quantity must be positive"))
	}
	price := req.Msg.PricePerUnit
	if price < 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("price_per_unit must be at least 1 coin"))
	}
	sold := p.SellResource(resourceID, qty)
	if sold < qty {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient resource"))
	}
	orderID := generateOrderID()
	s.store.AddOrder(&store.Order{
		OrderID:      orderID,
		PlayerID:     p.ID,
		OrderType:    store.OrderTypeSell,
		ResourceID:   resourceID,
		Quantity:     qty,
		PricePerUnit: price,
		CreatedAt:    time.Now(),
	})
	return connect.NewResponse(&gamev1.PlaceSellOrderResponse{
		State:  s.playerState(p),
		OrderId: orderID,
	}), nil
}

func (s *GameServer) PlaceBuyOrder(ctx context.Context, req *connect.Request[gamev1.PlaceBuyOrderRequest]) (*connect.Response[gamev1.PlaceBuyOrderResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	resourceID := req.Msg.ResourceId
	if resourceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("resource_id required"))
	}
	qty := req.Msg.Quantity
	if qty <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("quantity must be positive"))
	}
	price := req.Msg.PricePerUnit
	if price < 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("price_per_unit must be at least 1 coin"))
	}
	cost := qty * price
	if !p.SpendCoins(cost) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient coins"))
	}
	orderID := generateOrderID()
	s.store.AddOrder(&store.Order{
		OrderID:      orderID,
		PlayerID:     p.ID,
		OrderType:    store.OrderTypeBuy,
		ResourceID:   resourceID,
		Quantity:     qty,
		PricePerUnit: price,
		CreatedAt:    time.Now(),
	})
	return connect.NewResponse(&gamev1.PlaceBuyOrderResponse{
		State:   s.playerState(p),
		OrderId: orderID,
	}), nil
}

func (s *GameServer) FulfillSellOrder(ctx context.Context, req *connect.Request[gamev1.FulfillSellOrderRequest]) (*connect.Response[gamev1.FulfillSellOrderResponse], error) {
	buyer, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	orderID := req.Msg.OrderId
	o := s.store.RemoveOrder(orderID)
	if o == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("order not found"))
	}
	if o.OrderType != store.OrderTypeSell {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("order is not a sell order"))
	}
	if o.PlayerID == buyer.ID {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot fulfill your own sell order"))
	}
	cost := o.Quantity * o.PricePerUnit
	if !buyer.SpendCoins(cost) {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient coins"))
	}
	buyer.AddResource(o.ResourceID, o.Quantity)
	seller := s.store.GetPlayer(o.PlayerID)
	if seller != nil {
		seller.AddCoins(cost)
	}
	return connect.NewResponse(&gamev1.FulfillSellOrderResponse{
		State: s.playerState(buyer),
	}), nil
}

func (s *GameServer) FulfillBuyOrder(ctx context.Context, req *connect.Request[gamev1.FulfillBuyOrderRequest]) (*connect.Response[gamev1.FulfillBuyOrderResponse], error) {
	seller, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	orderID := req.Msg.OrderId
	o := s.store.RemoveOrder(orderID)
	if o == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("order not found"))
	}
	if o.OrderType != store.OrderTypeBuy {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("order is not a buy order"))
	}
	if o.PlayerID == seller.ID {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot fulfill your own buy order"))
	}
	sold := seller.SellResource(o.ResourceID, o.Quantity)
	if sold < o.Quantity {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("insufficient resource"))
	}
	earned := o.Quantity * o.PricePerUnit
	seller.AddCoins(earned)
	buyer := s.store.GetPlayer(o.PlayerID)
	if buyer != nil {
		buyer.AddResource(o.ResourceID, o.Quantity)
	}
	return connect.NewResponse(&gamev1.FulfillBuyOrderResponse{
		State: s.playerState(seller),
	}), nil
}

func (s *GameServer) CancelOrder(ctx context.Context, req *connect.Request[gamev1.CancelOrderRequest]) (*connect.Response[gamev1.CancelOrderResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	orderID := req.Msg.OrderId
	o := s.store.RemoveOrder(orderID)
	if o == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("order not found"))
	}
	if o.PlayerID != p.ID {
		s.store.AddOrder(o)
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("can only cancel your own order"))
	}
	if o.OrderType == store.OrderTypeSell {
		p.AddResource(o.ResourceID, o.Quantity)
	} else {
		p.AddCoins(o.Quantity * o.PricePerUnit)
	}
	return connect.NewResponse(&gamev1.CancelOrderResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) GetOrder(ctx context.Context, req *connect.Request[gamev1.GetOrderRequest]) (*connect.Response[gamev1.GetOrderResponse], error) {
	orderID := req.Msg.OrderId
	if orderID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("order_id required"))
	}
	o := s.store.GetOrder(orderID)
	if o == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("order not found"))
	}
	orderType := gamev1.OrderType_ORDER_TYPE_UNSPECIFIED
	if o.OrderType == store.OrderTypeSell {
		orderType = gamev1.OrderType_ORDER_TYPE_SELL
	} else if o.OrderType == store.OrderTypeBuy {
		orderType = gamev1.OrderType_ORDER_TYPE_BUY
	}
	totalCost := o.Quantity * o.PricePerUnit
	return connect.NewResponse(&gamev1.GetOrderResponse{
		OrderType:    orderType,
		ResourceId:   o.ResourceID,
		PricePerUnit: o.PricePerUnit,
		Quantity:     o.Quantity,
		TotalCost:    totalCost,
	}), nil
}

func roundTo1000(x int64) int64 {
	if x <= 0 {
		return 0
	}
	return int64(math.Round(float64(x)/1000)) * 1000
}

func (s *GameServer) GetLoans(ctx context.Context, req *connect.Request[gamev1.GetLoansRequest]) (*connect.Response[gamev1.GetLoansResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	loans := s.store.GetLoansByPlayer(p.ID)
	loanCount := len(loans)
	coins := p.GetCoins()
	proportions := []float64{0.25, 0.5, 1.0}
	offeredSet := make(map[int64]bool)
	var offeredAmounts []int64
	for _, prop := range proportions {
		if loanCount >= s.cfg.LoanMaxCount {
			break
		}
		amount := roundTo1000(int64(float64(coins) * prop))
		if amount < 1000 {
			continue
		}
		if offeredSet[amount] {
			continue
		}
		offeredSet[amount] = true
		offeredAmounts = append(offeredAmounts, amount)
		if len(offeredAmounts) >= s.cfg.LoanMaxCount {
			break
		}
	}
	protoLoans := make([]*gamev1.Loan, 0, len(loans))
	for _, l := range loans {
		protoLoans = append(protoLoans, &gamev1.Loan{
			LoanId:         l.LoanID,
			Balance:        l.Balance,
			InterestRate:   l.InterestRate,
			CreatedAt:      l.CreatedAt.Unix(),
			OriginalAmount: l.OriginalAmount,
		})
	}
	return connect.NewResponse(&gamev1.GetLoansResponse{
		Loans:          protoLoans,
		OfferedAmounts: offeredAmounts,
	}), nil
}

func (s *GameServer) TakeLoan(ctx context.Context, req *connect.Request[gamev1.TakeLoanRequest]) (*connect.Response[gamev1.TakeLoanResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	amount := req.Msg.Amount
	if amount < 1000 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount must be at least 1000"))
	}
	if s.store.LoanCountByPlayer(p.ID) >= s.cfg.LoanMaxCount {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("max loans reached"))
	}
	coins := p.GetCoins()
	proportions := []float64{0.25, 0.5, 1.0}
	valid := false
	for _, prop := range proportions {
		if roundTo1000(int64(float64(coins)*prop)) == amount {
			valid = true
			break
		}
	}
	if !valid {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount must be one of the offered amounts"))
	}
	loanID := uuid.New().String()
	s.store.AddLoan(&store.Loan{
		LoanID:         loanID,
		PlayerID:       p.ID,
		Balance:        amount,
		InterestRate:   s.cfg.LoanInterestRate,
		CreatedAt:      time.Now(),
		OriginalAmount: amount,
	})
	return connect.NewResponse(&gamev1.TakeLoanResponse{
		State: s.playerState(p),
	}), nil
}

func (s *GameServer) PayOffLoan(ctx context.Context, req *connect.Request[gamev1.PayOffLoanRequest]) (*connect.Response[gamev1.PayOffLoanResponse], error) {
	p, err := s.getOrCreatePlayer(ctx)
	if err != nil {
		return nil, err
	}
	loanID := req.Msg.LoanId
	amount := req.Msg.Amount
	if loanID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("loan_id required"))
	}
	if amount <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("amount must be positive"))
	}
	_, _ = s.store.PayOffLoan(loanID, p.ID, amount)
	return connect.NewResponse(&gamev1.PayOffLoanResponse{
		State: s.playerState(p),
	}), nil
}

func authMiddleware(checker AuthChecker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" && r.Method == http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		user := checker.AuthFromHttpReq(r)
		ctx := context.WithValue(r.Context(), authUserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func playerIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get(PlayerIDHeader); id != "" {
			ctx := context.WithValue(r.Context(), PlayerIDHeader, id)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// OAuth2RouteRegistrar registers OAuth2 login/callback routes onto the mux.
type OAuth2RouteRegistrar interface {
	HandleOAuthLogin(http.ResponseWriter, *http.Request)
	HandleOAuthCallback(http.ResponseWriter, *http.Request)
}

// LocalLoginHandler handles POST /login for username/password authentication.
// Call NewLocalLoginHandler with a non-nil AuthShimContext when local users are enabled.
type LocalLoginHandler struct {
	AuthCtx *auth.AuthShimContext
}

// NewLocalLoginHandler returns a handler for POST /login. AuthCtx must be non-nil and have LocalUsers enabled.
func NewLocalLoginHandler(authCtx *auth.AuthShimContext) http.Handler {
	return &LocalLoginHandler{AuthCtx: authCtx}
}

func (h *LocalLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Username == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "username and password required"})
		return
	}
	if !haslocal.CheckUserPassword(h.AuthCtx.Config, req.Username, req.Password) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}
	sidBytes := make([]byte, 32)
	if _, err := rand.Read(sidBytes); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sid := hex.EncodeToString(sidBytes)
	h.AuthCtx.RegisterUserSession("local", sid, req.Username)
	cookieName := h.AuthCtx.Config.GetLocalSessionCookieName()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sid,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

const npcInitialCoins = 1e12
const priceModifierCap = 0.3

// RunNPCTick picks a random NPC and resource and, with probability from buy_eagerness/sell_eagerness, places one buy and one sell order. Price modifiers use wealth and risk (e.g. high risk sells above market).
func RunNPCTick(str *store.Store, cfg *config.Config) {
	if len(cfg.NPCs) == 0 {
		return
	}
	npc := &cfg.NPCs[mrand.Intn(len(cfg.NPCs))]
	if len(npc.Resources) == 0 {
		return
	}
	resCfg := &npc.Resources[mrand.Intn(len(npc.Resources))]
	if gamedata.ResourceByID(cfg.Resources, resCfg.ResourceType) == nil {
		return
	}
	npcPlayer := str.GetOrCreateNPCPlayer(npc.Name, npcInitialCoins)
	marketPrice := str.GetMarketPrice(resCfg.ResourceType)

	if resCfg.BuyEagerness > 0 && resCfg.MaxCapacity >= 1 && mrand.Intn(100) < resCfg.BuyEagerness {
		quantity := 1 + mrand.Int63n(resCfg.MaxCapacity)
		buyMod := float64(npc.Wealth)/100*0.15 - float64(npc.Risk)/100*0.1
		if buyMod > priceModifierCap {
			buyMod = priceModifierCap
		}
		if buyMod < -priceModifierCap {
			buyMod = -priceModifierCap
		}
		price := int64(float64(marketPrice) * (1 + buyMod))
		if price < 0 {
			price = 0
		}
		cost := quantity * price
		if npcPlayer.SpendCoins(cost) {
			orderID := generateOrderID()
			str.AddOrder(&store.Order{
				OrderID:      orderID,
				PlayerID:     npcPlayer.ID,
				OrderType:    store.OrderTypeBuy,
				ResourceID:   resCfg.ResourceType,
				Quantity:     quantity,
				PricePerUnit: price,
				CreatedAt:    time.Now(),
			})
		}
	}

	if resCfg.SellEagerness > 0 && resCfg.MaxCapacity >= 1 {
		if mrand.Intn(100) >= resCfg.SellEagerness {
			return
		}
		have := npcPlayer.GetResource(resCfg.ResourceType)
		if have <= 0 {
			return
		}
		maxQty := resCfg.MaxCapacity
		if have < maxQty {
			maxQty = have
		}
		quantity := 1 + mrand.Int63n(maxQty)
		sold := npcPlayer.SellResource(resCfg.ResourceType, quantity)
		if sold == 0 {
			return
		}
		sellMod := float64(npc.Risk)/100*0.2 - float64(npc.Wealth)/100*0.1
		if sellMod > priceModifierCap {
			sellMod = priceModifierCap
		}
		if sellMod < -priceModifierCap {
			sellMod = -priceModifierCap
		}
		price := int64(float64(marketPrice) * (1 + sellMod))
		if price < 0 {
			price = 0
		}
		orderID := generateOrderID()
		str.AddOrder(&store.Order{
			OrderID:      orderID,
			PlayerID:     npcPlayer.ID,
			OrderType:    store.OrderTypeSell,
			ResourceID:   resCfg.ResourceType,
			Quantity:     sold,
			PricePerUnit: price,
			CreatedAt:    time.Now(),
		})
	}
}

func NewMux(store *store.Store, cfg *config.Config, log *logrus.Logger, auth AuthChecker, oauth2 OAuth2RouteRegistrar, loginHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	if oauth2 != nil {
		mux.HandleFunc("/oauth/login", oauth2.HandleOAuthLogin)
		mux.HandleFunc("/oauth/callback", oauth2.HandleOAuthCallback)
	}
	if loginHandler != nil {
		mux.Handle("POST /login", loginHandler)
	}
	worldSvc := NewWorldServer(store, cfg, log)
	gameSvc := NewGameServer(store, cfg, log)
	mux.Handle(gamev1connect.NewWorldServiceHandler(worldSvc))
	mux.Handle(gamev1connect.NewGameServiceHandler(gameSvc))
	h := playerIDMiddleware(mux)
	if auth != nil {
		h = authMiddleware(auth, h)
	}
	return h
}
