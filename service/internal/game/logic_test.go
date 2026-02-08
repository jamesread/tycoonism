package game

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"webfarm/internal/config"
	"webfarm/internal/gamedata"
)

func TestUpgradeDuration(t *testing.T) {
	cfg := &config.Config{
		UpgradeBaseDur:   time.Minute,
		UpgradeTimeStep:  time.Minute,
		UpgradeMaxLevel:  10,
	}
	assert.Equal(t, time.Duration(0), UpgradeDuration(cfg, 0, 1.0))
	assert.Equal(t, time.Minute, UpgradeDuration(cfg, 1, 1.0))
	assert.Equal(t, 2*time.Minute, UpgradeDuration(cfg, 2, 1.0))
	assert.Equal(t, 9*time.Minute, UpgradeDuration(cfg, 9, 1.0))
	assert.Equal(t, time.Duration(0), UpgradeDuration(cfg, 10, 1.0))

	// upgrade_time_factor 1.3: level 1 -> 1min, level 2 -> 2*1.3 = 2.6min
	d := UpgradeDuration(cfg, 2, 1.3)
	assert.Equal(t, time.Duration(2.6*float64(time.Minute)), d)
}

func TestUpgradeCost(t *testing.T) {
	cfg := &config.Config{PlacementCost: 10, UpgradeMaxLevel: 10}
	assert.Equal(t, int64(0), UpgradeCost(cfg, 1))
	assert.Equal(t, int64(7), UpgradeCost(cfg, 2))
	assert.Equal(t, int64(9), UpgradeCost(cfg, 3))
	assert.Equal(t, int64(12), UpgradeCost(cfg, 4))
	assert.Equal(t, int64(57), UpgradeCost(cfg, 10))
	assert.Equal(t, int64(0), UpgradeCost(cfg, 11))
}

func TestResourceProducedByBuilding(t *testing.T) {
	buildings := []gamedata.BuildingDef{
		{
			ID:            "wheat_farm",
			TickResources: map[string]int64{"wheat": 2},
			UpgradeProductionFactor: map[string]float64{"wheat": 1.0},
		},
		{
			ID:            "stone_mine",
			TickResources: map[string]int64{"stone": 3},
			UpgradeProductionFactor: map[string]float64{"stone": 1.0},
		},
	}
	p := ResourceProducedByBuilding("wheat_farm", 1, buildings)
	require.NotNil(t, p)
	assert.Equal(t, int64(2), p["wheat"])
	assert.Equal(t, int64(0), p["stone"])

	p = ResourceProducedByBuilding("wheat_farm", 3, buildings)
	require.NotNil(t, p)
	assert.Equal(t, int64(6), p["wheat"])

	p = ResourceProducedByBuilding("stone_mine", 2, buildings)
	require.NotNil(t, p)
	assert.Equal(t, int64(6), p["stone"]) // base 3 * level 2 (linear)

	p = ResourceProducedByBuilding("unknown", 1, buildings)
	assert.Nil(t, p)
}

func TestTotalUpgradeTimeToLevel(t *testing.T) {
	cfg := &config.Config{
		UpgradeBaseDur:   time.Minute,
		UpgradeTimeStep:  time.Minute,
		UpgradeMaxLevel:  10,
	}
	assert.Equal(t, time.Duration(0), TotalUpgradeTimeToLevel(cfg, 1, 1.0))
	assert.Equal(t, time.Minute, TotalUpgradeTimeToLevel(cfg, 2, 1.0))           // level 1->2
	assert.Equal(t, 3*time.Minute, TotalUpgradeTimeToLevel(cfg, 3, 1.0))         // 1 + 2
	assert.Equal(t, 6*time.Minute, TotalUpgradeTimeToLevel(cfg, 4, 1.0))         // 1+2+3
}

func TestSellDuration(t *testing.T) {
	cfg := &config.Config{
		UpgradeBaseDur:   time.Minute,
		UpgradeTimeStep:  time.Minute,
		UpgradeMaxLevel:  10,
	}
	assert.Equal(t, time.Duration(0), SellDuration(cfg, 1, 1.0))
	assert.Equal(t, time.Duration(0.30*float64(time.Minute)), SellDuration(cfg, 2, 1.0))
	assert.Equal(t, time.Duration(0.30*3*float64(time.Minute)), SellDuration(cfg, 3, 1.0))
}

func TestSellRefund(t *testing.T) {
	cfg := &config.Config{PlacementCost: 10}
	assert.Equal(t, int64(0), SellRefund(cfg, 0))
	assert.Equal(t, int64(5), SellRefund(cfg, 1))
	assert.Equal(t, int64(10), SellRefund(cfg, 2))
	assert.Equal(t, int64(15), SellRefund(cfg, 3))
}
