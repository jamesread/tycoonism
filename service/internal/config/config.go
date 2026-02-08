package config

import (
	"fmt"
	"sort"
	"time"

	"github.com/jamesread/httpauthshim/authpublic"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
	"webfarm/internal/gamedata"
)

// parseDateTime parses a date string as RFC3339 or YYYY-MM-DD (midnight UTC). Used for world_starts_at / world_ends_at.
func parseDateTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("invalid date: %q (use RFC3339 or YYYY-MM-DD)", s)
}

const (
	DefaultTickInterval        = 1 * time.Minute
	DefaultDaysPerCycle        = 7
	DefaultStartingCoins       = 100 // new players start with 100 gold (coins)
	DefaultAngelInvestorCoins  = 100 // grant when player has 0 coins and 0 buildings
	DefaultPlacementCost       = 10
	DefaultUpgradeBaseDur      = 1 * time.Minute
	DefaultUpgradeTimeStep     = 1 * time.Minute
	DefaultUpgradeMaxLevel     = 10
	DefaultUpgradeCostBase     = 5
	DefaultListenAddr          = ":4838"
	DefaultStaticDir           = ""
	DefaultStateFile           = "state.yml"
	DefaultStateBackupInterval = 5 * time.Minute
	DefaultResourcesFile       = "data/resources.yaml"
	DefaultBuildingsFile       = "data/buildings.yaml"
	DefaultNPCsFile            = "data/npcs.yaml"
	DefaultLoanInterestRate    = 0.01  // 1% per tick
	DefaultLoanMaxCount        = 3
)

type Config struct {
	TickInterval        time.Duration
	DaysPerCycle        int
	StartingCoins       int64
	StartingResources   map[string]int64 // resource_id -> starting quantity for new players
	AngelInvestorCoins  int64
	PlacementCost       int64
	PlacementCostByKey  map[string]int64 // placement_key -> cost; used when building has placement_key set
	UpgradeBaseDur      time.Duration
	UpgradeTimeStep     time.Duration
	UpgradeMaxLevel     int
	UpgradeCostBase     int64
	ListenAddr          string
	StaticDir           string
	StateFile           string
	StateBackupInterval time.Duration
	ResourcesFile       string
	BuildingsFile       string
	NPCsFile            string
	Resources           []gamedata.ResourceDef
	Buildings           []gamedata.BuildingDef
	NPCs                []gamedata.NPCDef
	Auth                *authpublic.Config
	WorldStartsAt       time.Time // Game world start (date string in config: game.world_starts_at, RFC3339 or YYYY-MM-DD); zero = not set
	WorldEndsAt         time.Time // Game world end (date string in config: game.world_ends_at, RFC3339 or YYYY-MM-DD); zero = no end
	LoanInterestRate    float64   // Interest rate per tick (e.g. 0.01 = 1%)
	LoanMaxCount        int       // Max number of loans a player can have (e.g. 3)
}

// WorldDuration returns the duration from world start to world end. Zero if either is not set.
func (c *Config) WorldDuration() time.Duration {
	if c.WorldStartsAt.IsZero() || c.WorldEndsAt.IsZero() {
		return 0
	}
	return c.WorldEndsAt.Sub(c.WorldStartsAt)
}

func Load(k *koanf.Koanf) *Config {
	c := &Config{
		TickInterval:        DefaultTickInterval,
		DaysPerCycle:        DefaultDaysPerCycle,
		StartingCoins:       DefaultStartingCoins,
		StartingResources:   map[string]int64{},
		AngelInvestorCoins:  DefaultAngelInvestorCoins,
		PlacementCost:       DefaultPlacementCost,
		UpgradeBaseDur:      DefaultUpgradeBaseDur,
		UpgradeTimeStep:     DefaultUpgradeTimeStep,
		UpgradeMaxLevel:     DefaultUpgradeMaxLevel,
		UpgradeCostBase:     DefaultUpgradeCostBase,
		ListenAddr:          DefaultListenAddr,
		StaticDir:           DefaultStaticDir,
		StateFile:           DefaultStateFile,
		StateBackupInterval: DefaultStateBackupInterval,
		ResourcesFile:       DefaultResourcesFile,
		BuildingsFile:       DefaultBuildingsFile,
		NPCsFile:            DefaultNPCsFile,
		LoanInterestRate:    DefaultLoanInterestRate,
		LoanMaxCount:        DefaultLoanMaxCount,
	}
	if s := k.String("game.world_starts_at"); s != "" {
		if t, err := parseDateTime(s); err == nil {
			c.WorldStartsAt = t
		}
	}
	if s := k.String("game.world_ends_at"); s != "" {
		if t, err := parseDateTime(s); err == nil {
			c.WorldEndsAt = t
		}
	}
	if v := k.Duration("simulation.tick_interval"); v > 0 {
		c.TickInterval = v
	}
	if v := k.Int("simulation.days_per_cycle"); v > 0 {
		c.DaysPerCycle = v
	}
	if v := k.Int64("game.starting_coins"); v >= 0 {
		c.StartingCoins = v
	}
	if v := k.Int64("game.starting_wheat"); v >= 0 {
		if c.StartingResources == nil {
			c.StartingResources = map[string]int64{}
		}
		c.StartingResources["wheat"] = v
	}
	if v := k.Int64("game.starting_stone"); v >= 0 {
		if c.StartingResources == nil {
			c.StartingResources = map[string]int64{}
		}
		c.StartingResources["stone"] = v
	}
	if v := k.Int64("game.angel_investor_coins"); v > 0 {
		c.AngelInvestorCoins = v
	}
	if k.Exists("game.placement_cost") {
		if v := k.Int64("game.placement_cost"); v >= 0 {
			c.PlacementCost = v
		}
	}
	if k.Exists("game.placement_cost_by_key") {
		var byKey map[string]int64
		if err := k.Unmarshal("game.placement_cost_by_key", &byKey); err == nil && byKey != nil {
			c.PlacementCostByKey = byKey
		}
	}
	if v := k.Duration("game.upgrade.base_duration"); v > 0 {
		c.UpgradeBaseDur = v
	}
	if v := k.Duration("game.upgrade.time_step"); v > 0 {
		c.UpgradeTimeStep = v
	}
	if v := k.Int("game.upgrade.max_level"); v > 0 {
		c.UpgradeMaxLevel = v
	}
	if v := k.Int64("game.upgrade.cost_base"); v >= 0 {
		c.UpgradeCostBase = v
	}
	if s := k.String("server.listen_addr"); s != "" {
		c.ListenAddr = s
	}
	if s := k.String("server.static_dir"); s != "" {
		c.StaticDir = s
	}
	if s := k.String("state.file"); s != "" {
		c.StateFile = s
	}
	if v := k.Duration("state.backup_interval"); v > 0 {
		c.StateBackupInterval = v
	}
	if s := k.String("data.resources_file"); s != "" {
		c.ResourcesFile = s
	}
	if s := k.String("data.buildings_file"); s != "" {
		c.BuildingsFile = s
	}
	if s := k.String("data.npcs_file"); s != "" {
		c.NPCsFile = s
	}
	if k.Exists("game.loan_interest_rate") {
		if v := k.Float64("game.loan_interest_rate"); v >= 0 {
			c.LoanInterestRate = v
		}
	}
	if v := k.Int("game.loan_max_count"); v > 0 {
		c.LoanMaxCount = v
	}
	if k.Exists("auth") {
		authCfg := &authpublic.Config{}
		if err := k.Unmarshal("auth", authCfg); err == nil {
			c.Auth = authCfg
		}
	}
	return c
}

var validTopLevelKeys = map[string]bool{
	"simulation": true, "game": true, "server": true, "state": true, "data": true, "auth": true,
}

// LogLoadSummary logs config load summary: auth present, OAuth2 providers, world time window, and any unrecognized top-level keys.
func LogLoadSummary(log *logrus.Logger, k *koanf.Koanf, c *Config) {
	if !c.WorldStartsAt.IsZero() {
		log.WithField("world_starts_at", c.WorldStartsAt.Format(time.RFC3339)).WithField("unix", c.WorldStartsAt.Unix()).Info("world start time configured")
	}
	if !c.WorldEndsAt.IsZero() {
		log.WithField("world_ends_at", c.WorldEndsAt.Format(time.RFC3339)).WithField("unix", c.WorldEndsAt.Unix()).Info("world end time configured")
	}
	if c.Auth != nil {
		log.Info("auth config loaded")
		if n := len(c.Auth.OAuth2Providers); n > 0 {
			names := make([]string, 0, n)
			for name := range c.Auth.OAuth2Providers {
				names = append(names, name)
			}
			sort.Strings(names)
			log.WithField("providers", names).Info("oauth2 providers found")
		}
		if c.Auth.LocalUsers.Enabled {
			log.WithField("count", len(c.Auth.LocalUsers.Users)).Info("local users enabled")
		}
	}
	raw := k.Raw()
	if raw == nil {
		return
	}
	var invalid []string
	for key := range raw {
		if !validTopLevelKeys[key] {
			invalid = append(invalid, key)
		}
	}
	if len(invalid) > 0 {
		sort.Strings(invalid)
		log.WithField("keys", invalid).Warn("unrecognized config key(s), ignored")
	}
}

// LoadGameData loads resources and buildings from configured paths and validates them.
// Call after Load() to populate Resources and Buildings. Returns error if invalid.
func (c *Config) LoadGameData() error {
	resources, err := gamedata.LoadResources(c.ResourcesFile)
	if err != nil {
		return err
	}
	buildings, err := gamedata.LoadBuildings(c.BuildingsFile)
	if err != nil {
		return err
	}
	if err := gamedata.Validate(resources, buildings); err != nil {
		return err
	}
	npcs, err := gamedata.LoadNPCs(c.NPCsFile)
	if err != nil {
		return err
	}
	if len(npcs) > 0 {
		if err := gamedata.ValidateNPCs(resources, npcs); err != nil {
			return err
		}
	}
	c.Resources = resources
	c.Buildings = buildings
	c.NPCs = npcs
	if c.StartingResources == nil {
		c.StartingResources = map[string]int64{}
	}
	defaultStarting := map[string]int64{"wheat": 50, "stone": 50}
	for _, r := range resources {
		if _, ok := c.StartingResources[r.ID]; !ok {
			if v, hasDefault := defaultStarting[r.ID]; hasDefault {
				c.StartingResources[r.ID] = v
			} else {
				c.StartingResources[r.ID] = 0
			}
		}
	}
	return nil
}

// PlacementCostForBuilding returns the coin cost to place the given building type.
// Uses PlacementCostByKey[def.PlacementKey] when the building has a placement_key and it is configured;
// otherwise falls back to PlacementCost. Empty placement_key is treated as "default".
func (c *Config) PlacementCostForBuilding(def *gamedata.BuildingDef) int64 {
	key := def.PlacementKey
	if key == "" {
		key = "default"
	}
	if c.PlacementCostByKey != nil {
		if cost, ok := c.PlacementCostByKey[key]; ok {
			return cost
		}
	}
	return c.PlacementCost
}
