package starterdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Whether every entry builds into valid records is checked against the real units in
// sql.TestCatalogSqlRepository; this checks the file reads and its keys are unique.
func TestLoad(t *testing.T) {
	data, err := Load()

	require.NoError(t, err)
	require.Len(t, data.Categories, 10)
	require.Len(t, data.Items, 32)

	keys := map[string]bool{}
	for _, c := range data.Categories {
		require.False(t, keys["category:"+c.Key], "category %q twice", c.Key)
		keys["category:"+c.Key] = true
	}
	for _, i := range data.Items {
		require.False(t, keys["item:"+i.Key], "item %q twice", i.Key)
		keys["item:"+i.Key] = true
	}

	var garlic = data.Items[2]
	require.Equal(t, "garlic", garlic.Key)
	require.Equal(t, []string{"head", "g"}, garlic.Buy)
	require.Len(t, garlic.Conversions, 2)
	require.Equal(t, "head", garlic.Conversions[0].From)
	require.Equal(t, "clove", garlic.Conversions[0].To)
	require.InDelta(t, 10.0, garlic.Conversions[0].Factor, 0)
}
