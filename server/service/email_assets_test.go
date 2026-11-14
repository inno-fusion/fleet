package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailAssetURLHasNoVendorDefault(t *testing.T) {
	require.Empty(t, getAssetURL())
}
