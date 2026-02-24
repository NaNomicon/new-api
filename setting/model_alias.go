package setting

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"

	"github.com/QuantumNous/new-api/common"
)


type ModelAliasTarget struct {
	Model    string `json:"model"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
}


type ModelAlias struct {
	Alias   string             `json:"alias"`
	Targets []ModelAliasTarget `json:"targets"`
}

var (
	modelAliases  []ModelAlias
	modelAliasMap map[string]*ModelAlias
	modelAliasMu  sync.RWMutex
)

func init() {
	modelAliasMap = make(map[string]*ModelAlias)
}


func UpdateModelAliasesByJSONString(jsonString string) error {
	var aliases []ModelAlias
	if err := common.Unmarshal([]byte(jsonString), &aliases); err != nil {
		return fmt.Errorf("failed to parse model aliases: %w", err)
	}
	if err := validateModelAliases(aliases); err != nil {
		return err
	}
	modelAliasMu.Lock()
	defer modelAliasMu.Unlock()
	modelAliases = aliases
	rebuildAliasMapLocked()
	return nil
}


func ModelAliases2JSONString() string {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	jsonBytes, err := common.Marshal(modelAliases)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}


func GetModelAliases() []ModelAlias {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	result := make([]ModelAlias, len(modelAliases))
	copy(result, modelAliases)
	return result
}


func GetModelAlias(aliasName string) *ModelAlias {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	return modelAliasMap[aliasName]
}


func IsModelAlias(modelName string) bool {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	_, ok := modelAliasMap[modelName]
	return ok
}

// ResolveAlias picks an upstream model for aliasName, excluding any model
// already present in failedModels. It selects from the lowest-numbered
// priority tier first; within a tier it uses weighted random sampling.
// Returns "" when no eligible targets remain.
func ResolveAlias(aliasName string, failedModels map[string]bool) string {
	modelAliasMu.RLock()
	alias := modelAliasMap[aliasName]
	modelAliasMu.RUnlock()

	if alias == nil || len(alias.Targets) == 0 {
		return ""
	}

	var available []ModelAliasTarget
	for _, t := range alias.Targets {
		if !failedModels[t.Model] {
			available = append(available, t)
		}
	}
	if len(available) == 0 {
		return ""
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].Priority < available[j].Priority
	})

	topPriority := available[0].Priority
	var tier []ModelAliasTarget
	for _, t := range available {
		if t.Priority == topPriority {
			tier = append(tier, t)
		} else {
			break
		}
	}

	return weightedRandomPick(tier)
}

func weightedRandomPick(tier []ModelAliasTarget) string {
	if len(tier) == 0 {
		return ""
	}
	if len(tier) == 1 {
		return tier[0].Model
	}

	totalWeight := 0
	for _, t := range tier {
		w := t.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	r := rand.Intn(totalWeight)
	for _, t := range tier {
		w := t.Weight
		if w <= 0 {
			w = 1
		}
		r -= w
		if r < 0 {
			return t.Model
		}
	}
	return tier[0].Model
}

// rebuildAliasMapLocked must be called with modelAliasMu held.
func rebuildAliasMapLocked() {
	modelAliasMap = make(map[string]*ModelAlias, len(modelAliases))
	for i := range modelAliases {
		modelAliasMap[modelAliases[i].Alias] = &modelAliases[i]
	}
}

func validateModelAliases(aliases []ModelAlias) error {
	seen := make(map[string]bool)
	for _, a := range aliases {
		if a.Alias == "" {
			return fmt.Errorf("model alias name cannot be empty")
		}
		if seen[a.Alias] {
			return fmt.Errorf("duplicate model alias: %s", a.Alias)
		}
		seen[a.Alias] = true
		if len(a.Targets) == 0 {
			return fmt.Errorf("model alias %q has no targets", a.Alias)
		}
		targetModels := make(map[string]bool)
		for _, t := range a.Targets {
			if t.Model == "" {
				return fmt.Errorf("model alias %q has a target with empty model name", a.Alias)
			}
			if t.Model == a.Alias {
				return fmt.Errorf("model alias %q has a self-referencing target", a.Alias)
			}
			if targetModels[t.Model] {
				return fmt.Errorf("model alias %q has duplicate target model: %s", a.Alias, t.Model)
			}
			targetModels[t.Model] = true
		}
	}

	aliasNames := make(map[string]bool, len(aliases))
	for _, a := range aliases {
		aliasNames[a.Alias] = true
	}
	for _, a := range aliases {
		for _, t := range a.Targets {
			if aliasNames[t.Model] {
				return fmt.Errorf("model alias %q targets another alias %q (cyclic references not allowed)", a.Alias, t.Model)
			}
		}
	}
	return nil
}
