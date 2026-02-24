package setting

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupAliases(t *testing.T, raw string) {
	t.Helper()
	require.NoError(t, UpdateModelAliasesByJSONString(raw))
	t.Cleanup(func() {
		_ = UpdateModelAliasesByJSONString("[]")
	})
}

func TestUpdateModelAliasesByJSONString_Valid(t *testing.T) {
	setupAliases(t, `[{"alias":"role-planner","targets":[{"model":"model-a","priority":1,"weight":60},{"model":"model-b","priority":1,"weight":40}]}]`)
	aliases := GetModelAliases()
	require.Len(t, aliases, 1)
	require.Equal(t, "role-planner", aliases[0].Alias)
	require.Len(t, aliases[0].Targets, 2)
}

func TestUpdateModelAliasesByJSONString_InvalidJSON(t *testing.T) {
	err := UpdateModelAliasesByJSONString("not-json")
	require.Error(t, err)
}

func TestUpdateModelAliasesByJSONString_EmptyAlias(t *testing.T) {
	err := UpdateModelAliasesByJSONString(`[{"alias":"","targets":[{"model":"m","priority":1,"weight":1}]}]`)
	require.Error(t, err)
}

func TestUpdateModelAliasesByJSONString_NoDuplicateAlias(t *testing.T) {
	err := UpdateModelAliasesByJSONString(`[
		{"alias":"a","targets":[{"model":"m1","priority":1,"weight":1}]},
		{"alias":"a","targets":[{"model":"m2","priority":1,"weight":1}]}
	]`)
	require.Error(t, err)
}

func TestUpdateModelAliasesByJSONString_SelfReference(t *testing.T) {
	err := UpdateModelAliasesByJSONString(`[{"alias":"role-planner","targets":[{"model":"role-planner","priority":1,"weight":1}]}]`)
	require.Error(t, err)
}

func TestUpdateModelAliasesByJSONString_CyclicReference(t *testing.T) {
	err := UpdateModelAliasesByJSONString(`[
		{"alias":"alias-a","targets":[{"model":"alias-b","priority":1,"weight":1}]},
		{"alias":"alias-b","targets":[{"model":"model-x","priority":1,"weight":1}]}
	]`)
	require.Error(t, err)
}

func TestIsModelAlias(t *testing.T) {
	setupAliases(t, `[{"alias":"role-researcher","targets":[{"model":"model-x","priority":1,"weight":1}]}]`)
	require.True(t, IsModelAlias("role-researcher"))
	require.False(t, IsModelAlias("model-x"))
	require.False(t, IsModelAlias("unknown"))
}

func TestResolveAlias_PicksLowestPriority(t *testing.T) {
	setupAliases(t, `[{"alias":"role-planner","targets":[
		{"model":"cheap-model","priority":2,"weight":100},
		{"model":"smart-model","priority":1,"weight":100}
	]}]`)
	for i := 0; i < 20; i++ {
		got := ResolveAlias("role-planner", nil)
		require.Equal(t, "smart-model", got, "priority 1 should always win over priority 2")
	}
}

func TestResolveAlias_ExcludesFailedModels(t *testing.T) {
	setupAliases(t, `[{"alias":"role-planner","targets":[
		{"model":"model-a","priority":1,"weight":100},
		{"model":"model-b","priority":2,"weight":100}
	]}]`)
	failed := map[string]bool{"model-a": true}
	got := ResolveAlias("role-planner", failed)
	require.Equal(t, "model-b", got)
}

func TestResolveAlias_AllExhausted(t *testing.T) {
	setupAliases(t, `[{"alias":"role-planner","targets":[
		{"model":"model-a","priority":1,"weight":100}
	]}]`)
	got := ResolveAlias("role-planner", map[string]bool{"model-a": true})
	require.Equal(t, "", got)
}

func TestResolveAlias_UnknownAlias(t *testing.T) {
	setupAliases(t, "[]")
	require.Equal(t, "", ResolveAlias("nonexistent", nil))
}

func TestResolveAlias_WeightedDistribution(t *testing.T) {
	setupAliases(t, `[{"alias":"role-planner","targets":[
		{"model":"heavy","priority":1,"weight":90},
		{"model":"light","priority":1,"weight":10}
	]}]`)

	counts := map[string]int{}
	const iterations = 2000
	for i := 0; i < iterations; i++ {
		m := ResolveAlias("role-planner", nil)
		counts[m]++
	}
	require.Contains(t, counts, "heavy")
	require.Contains(t, counts, "light")
	heavyRatio := float64(counts["heavy"]) / iterations
	require.Greater(t, heavyRatio, 0.80, "heavy model should be selected ~90%% of the time")
	require.Less(t, heavyRatio, 0.98)
}

func TestResolveAlias_ZeroWeightDefaultsToOne(t *testing.T) {
	setupAliases(t, `[{"alias":"role-test","targets":[
		{"model":"m1","priority":1,"weight":0},
		{"model":"m2","priority":1,"weight":0}
	]}]`)
	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		counts[ResolveAlias("role-test", nil)]++
	}
	require.Contains(t, counts, "m1")
	require.Contains(t, counts, "m2")
}

func TestModelAliases2JSONString_RoundTrip(t *testing.T) {
	input := `[{"alias":"role-writer","targets":[{"model":"model-a","priority":1,"weight":100}]}]`
	setupAliases(t, input)

	out := ModelAliases2JSONString()
	var got []ModelAlias
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got, 1)
	require.Equal(t, "role-writer", got[0].Alias)
}

func TestGetModelAlias_ReturnsNilForUnknown(t *testing.T) {
	setupAliases(t, "[]")
	require.Nil(t, GetModelAlias("ghost"))
}
