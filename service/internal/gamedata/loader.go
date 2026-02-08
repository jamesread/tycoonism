package gamedata

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type ResourceDef struct {
	ID        string `yaml:"id"`
	Name      string `yaml:"name"`
	Icon      string `yaml:"icon"`
	BaseColor string `yaml:"base_color"`
}

// NPCResource defines per-resource AI factors for an NPC.
type NPCResource struct {
	ResourceType   string `yaml:"resource_type"`   // resource id
	BuyEagerness   int    `yaml:"buy_eagerness"`   // 0-100
	SellEagerness  int    `yaml:"sell_eagerness"`  // 0-100
	MaxCapacity    int64  `yaml:"max_capacity"`    // maximum quantity they will buy
}

// NPCDef is a non-player character with AI factors.
type NPCDef struct {
	Name     string       `yaml:"name"`
	Wealth   int          `yaml:"wealth"`   // 0-100
	Risk     int          `yaml:"risk"`     // 0-100
	Resources []NPCResource `yaml:"resources"`
}

type BuildingRequirement struct {
	BuildingID string `yaml:"building_id"`
	Count      int    `yaml:"count"`
}

type BuildingDef struct {
	ID                      string                 `yaml:"id"`
	Name                    string                 `yaml:"name"`
	Icon                    string                 `yaml:"icon"`
	BaseLevel               int32                  `yaml:"base_level"`
	TickResources           map[string]int64      `yaml:"tick_resources"`
	UpgradeProductionFactor map[string]float64    `yaml:"upgrade_production_factor"`
	UpgradeTimeFactor       float64                `yaml:"upgrade_time_factor"`
	Requirements            []BuildingRequirement  `yaml:"requirements"`
	PlacementKey            string                 `yaml:"placement_key"`
}

type resourcesFile struct {
	Resources []ResourceDef `yaml:"resources"`
}

type buildingsFile struct {
	Buildings []BuildingDef `yaml:"buildings"`
}

type npcsFile struct {
	NPCs []NPCDef `yaml:"npcs"`
}

func LoadResources(path string) ([]ResourceDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resources: %w", err)
	}
	var f resourcesFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse resources: %w", err)
	}
	seen := make(map[string]bool)
	for i := range f.Resources {
		r := &f.Resources[i]
		if r.ID == "" {
			return nil, fmt.Errorf("resources[%d]: id required", i)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("duplicate resource id: %s", r.ID)
		}
		seen[r.ID] = true
	}
	return f.Resources, nil
}

func LoadBuildings(path string) ([]BuildingDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read buildings: %w", err)
	}
	var f buildingsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse buildings: %w", err)
	}
	seen := make(map[string]bool)
	for i := range f.Buildings {
		b := &f.Buildings[i]
		if b.ID == "" {
			return nil, fmt.Errorf("buildings[%d]: id required", i)
		}
		if seen[b.ID] {
			return nil, fmt.Errorf("duplicate building id: %s", b.ID)
		}
		seen[b.ID] = true
		if b.BaseLevel < 1 {
			b.BaseLevel = 1
		}
		if b.UpgradeTimeFactor <= 0 {
			b.UpgradeTimeFactor = 1.0
		}
		if b.TickResources == nil {
			b.TickResources = make(map[string]int64)
		}
		if b.UpgradeProductionFactor == nil {
			b.UpgradeProductionFactor = make(map[string]float64)
		}
	}
	return f.Buildings, nil
}

// LoadNPCs loads NPC definitions from path. Optional file; returns nil slice if path is empty or file missing.
func LoadNPCs(path string) ([]NPCDef, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read npcs: %w", err)
	}
	var f npcsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse npcs: %w", err)
	}
	seen := make(map[string]bool)
	for i := range f.NPCs {
		n := &f.NPCs[i]
		if n.Name == "" {
			return nil, fmt.Errorf("npcs[%d]: name required", i)
		}
		if seen[n.Name] {
			return nil, fmt.Errorf("duplicate npc name: %s", n.Name)
		}
		seen[n.Name] = true
		if n.Wealth < 0 || n.Wealth > 100 {
			return nil, fmt.Errorf("npc %s: wealth must be 0-100", n.Name)
		}
		if n.Risk < 0 || n.Risk > 100 {
			return nil, fmt.Errorf("npc %s: risk must be 0-100", n.Name)
		}
		for j := range n.Resources {
			r := &n.Resources[j]
			if r.BuyEagerness < 0 || r.BuyEagerness > 100 {
				return nil, fmt.Errorf("npc %s resources[%d]: buy_eagerness must be 0-100", n.Name, j)
			}
			if r.SellEagerness < 0 || r.SellEagerness > 100 {
				return nil, fmt.Errorf("npc %s resources[%d]: sell_eagerness must be 0-100", n.Name, j)
			}
			if r.MaxCapacity < 0 {
				return nil, fmt.Errorf("npc %s resources[%d]: max_capacity must be >= 0", n.Name, j)
			}
		}
	}
	return f.NPCs, nil
}

func Validate(resources []ResourceDef, buildings []BuildingDef) error {
	resourceIDs := make(map[string]bool)
	for _, r := range resources {
		resourceIDs[r.ID] = true
	}
	buildingIDs := make(map[string]bool)
	for _, b := range buildings {
		buildingIDs[b.ID] = true
	}
	for _, b := range buildings {
		for resID := range b.TickResources {
			if !resourceIDs[resID] {
				return fmt.Errorf("building %s: tick_resources references unknown resource %s", b.ID, resID)
			}
		}
		for resID := range b.UpgradeProductionFactor {
			if !resourceIDs[resID] {
				return fmt.Errorf("building %s: upgrade_production_factor references unknown resource %s", b.ID, resID)
			}
		}
		for _, req := range b.Requirements {
			if !buildingIDs[req.BuildingID] {
				return fmt.Errorf("building %s: requirements references unknown building %s", b.ID, req.BuildingID)
			}
			if req.Count < 1 {
				return fmt.Errorf("building %s: requirement count must be >= 1", b.ID)
			}
		}
	}
	return nil
}

// ValidateNPCs checks that every NPC resource_type references a known resource.
func ValidateNPCs(resources []ResourceDef, npcs []NPCDef) error {
	resourceIDs := make(map[string]bool)
	for _, r := range resources {
		resourceIDs[r.ID] = true
	}
	for _, n := range npcs {
		for _, nr := range n.Resources {
			if !resourceIDs[nr.ResourceType] {
				return fmt.Errorf("npc %s: resource_type %q is not a known resource", n.Name, nr.ResourceType)
			}
		}
	}
	return nil
}

func ResourceByID(resources []ResourceDef, id string) *ResourceDef {
	for i := range resources {
		if resources[i].ID == id {
			return &resources[i]
		}
	}
	return nil
}

func BuildingByID(buildings []BuildingDef, id string) *BuildingDef {
	for i := range buildings {
		if buildings[i].ID == id {
			return &buildings[i]
		}
	}
	return nil
}

func NPCByName(npcs []NPCDef, name string) *NPCDef {
	for i := range npcs {
		if npcs[i].Name == name {
			return &npcs[i]
		}
	}
	return nil
}
