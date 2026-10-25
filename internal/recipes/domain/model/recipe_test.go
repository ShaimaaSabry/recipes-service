package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRecipe(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		inName   string
		wantName string
		wantErr  error
	}{
		{name: "valid", id: "recipe-1", inName: "Shakshuka", wantName: "Shakshuka"},
		{name: "tidies spaces in the name", id: "recipe-1", inName: "  Lentil   soup ", wantName: "Lentil soup"},
		{name: "blank name", id: "recipe-1", inName: "   ", wantErr: ErrInvalidRecipeName},
		{name: "blank id", id: " ", inName: "Soup", wantErr: ErrInvalidID},
		{name: "id too long", id: strings.Repeat("a", MaxIDLength+1), inName: "Soup", wantErr: ErrInvalidID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recipe, err := NewRecipe(tt.id, "device-1", tt.inName)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantName, recipe.Name())
			require.Equal(t, tt.id, recipe.ID())
			require.EqualValues(t, "device-1", recipe.Owner())
		})
	}
}
