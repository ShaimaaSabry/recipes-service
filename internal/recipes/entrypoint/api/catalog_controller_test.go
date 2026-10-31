package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchesETag(t *testing.T) {
	const etag = `"abc"`

	require.True(t, matchesETag(`"abc"`, etag))
	require.True(t, matchesETag(`W/"abc"`, etag), "proxies may weaken it")
	require.True(t, matchesETag(`"old", "abc"`, etag))
	require.True(t, matchesETag(`*`, etag))
	require.False(t, matchesETag(`"old"`, etag))
	require.False(t, matchesETag(``, etag))
}
