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
	if modelAliases == nil {
		return "[]"
	}
	s, _ := common.Marshal(modelAliases)
	return string(s)
}

func GetModelAliases() []ModelAlias {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	cp := make([]ModelAlias, len(modelAliases))
	copy(cp, modelAliases)
	return cp
}

func GetModelAlias(aliasName string) *ModelAlias {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	a := modelAliasMap[aliasName]
	return a
}

func IsModelAlias(modelName string) bool {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	_, ok := modelAliasMap[modelName]
	return ok
}

// ResolveAlias picks an upstream model for aliasName, excluding any models in
// failedModels. It selects within the lowest available priority tier using
// weighted-random sampling. Returns "" when all targets are exhausted or
// aliasName is not a registered alias.
func ResolveAlias(aliasName string, failedModels map[string]bool) string {
	modelAliasMu.RLock()
	defer modelAliasMu.RUnlock()
	alias, ok := modelAliasMap[aliasName]
	if !ok {
		return ""
	}
	byPriority := make(map[int][]ModelAliasTarget)
	for _, t := range alias.Targets {
		if !failedModels[t.Model] {
			byPriority[t.Priority] = append(byPriority[t.Priority], t)
		}
	}
	if len(byPriority) == 0 {
		return ""
	}

	tiers := make([]int, 0, len(byPriority))
	for p := range byPriority {
		tiers = append(tiers, p)
	}
	sort.Ints(tiers)

	return weightedRandomPick(byPriority[tiers[0]])
}

func weightedRandomPick(tier []ModelAliasTarget) string {
	if len(tier) == 1 {
		return tier[0].Model
	}
	total := 0
	for i := range tier {
		w := tier[i].Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	r := rand.Intn(total)
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
	m := make(map[string]*ModelAlias, len(modelAliases))
	for i := range modelAliases {
		m[modelAliases[i].Alias] = &modelAliases[i]
	}
	modelAliasMap = m
}

func validateModelAliases(aliases []ModelAlias) error {
	seen := make(map[string]bool, len(aliases))
	for _, a := range aliases {
		if a.Alias == "" {
			return fmt.Errorf("alias name must not be empty")
		}
		if seen[a.Alias] {
			return fmt.Errorf("duplicate alias %q", a.Alias)
		}
		seen[a.Alias] = true
	}
	for _, a := range aliases {
		for _, t := range a.Targets {
			if t.Model == a.Alias {
				return fmt.Errorf("alias %q cannot reference itself as a target", a.Alias)
			}
			if seen[t.Model] {
				return fmt.Errorf("alias %q: target %q is itself an alias; chaining aliases is not allowed", a.Alias, t.Model)
			}
		}
	}
	return nil
}
