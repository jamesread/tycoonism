package game

import (
	"math"
	"time"

	"webfarm/internal/config"
	"webfarm/internal/gamedata"
)

func UpgradeDuration(cfg *config.Config, fromLevel int32, upgradeTimeFactor float64) time.Duration {
	if fromLevel < 1 || fromLevel >= int32(cfg.UpgradeMaxLevel) {
		return 0
	}
	if upgradeTimeFactor <= 0 {
		upgradeTimeFactor = 1.0
	}
	base := cfg.UpgradeBaseDur + cfg.UpgradeTimeStep*time.Duration(fromLevel-1)
	mult := math.Pow(upgradeTimeFactor, float64(fromLevel-1))
	return time.Duration(float64(base) * mult)
}

// TotalUpgradeTimeToLevel returns the sum of upgrade durations from level 1 to level-1 (total time "built" to reach this level).
func TotalUpgradeTimeToLevel(cfg *config.Config, level int32, upgradeTimeFactor float64) time.Duration {
	if level <= 1 {
		return 0
	}
	var total time.Duration
	for from := int32(1); from < level; from++ {
		total += UpgradeDuration(cfg, from, upgradeTimeFactor)
	}
	return total
}

// SellDuration returns 30% of the total build time to reach the building's level.
func SellDuration(cfg *config.Config, level int32, upgradeTimeFactor float64) time.Duration {
	total := TotalUpgradeTimeToLevel(cfg, level, upgradeTimeFactor)
	return time.Duration(float64(total) * 0.30)
}

// SellRefund returns coins refunded when a building is sold (50% of placement cost per level).
func SellRefund(cfg *config.Config, level int32) int64 {
	if level < 1 {
		return 0
	}
	return cfg.PlacementCost * int64(level) / 2
}

// UpgradeCost returns coin cost to upgrade to toLevel. Base is 70% of building value (placement cost), increasing 30% per level.
func UpgradeCost(cfg *config.Config, toLevel int32) int64 {
	if toLevel < 2 || toLevel > int32(cfg.UpgradeMaxLevel) {
		return 0
	}
	base := float64(cfg.PlacementCost) * 0.70
	mult := math.Pow(1.30, float64(toLevel-2))
	return int64(math.Round(base * mult))
}

// ResourceProducedByBuilding returns resource_id -> quantity produced per tick for the given building type and level.
// When upgrade_production_factor is 1.0 or omitted, production is linear (base * level). Otherwise base * (factor ^ (level-1)).
func ResourceProducedByBuilding(typeID string, level int32, buildings []gamedata.BuildingDef) map[string]int64 {
	def := gamedata.BuildingByID(buildings, typeID)
	if def == nil || level < 1 {
		return nil
	}
	out := make(map[string]int64)
	for resID, baseAmount := range def.TickResources {
		factor := def.UpgradeProductionFactor[resID]
		var amount float64
		if factor <= 0 || factor == 1.0 {
			amount = float64(baseAmount) * float64(level)
		} else {
			amount = float64(baseAmount) * math.Pow(factor, float64(level-1))
		}
		out[resID] = int64(math.Round(amount))
	}
	return out
}
