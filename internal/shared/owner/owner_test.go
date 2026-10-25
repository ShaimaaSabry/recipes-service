package owner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    ID
		wantErr bool
	}{
		{name: "a device id", raw: "device-123", want: "device-123"},
		{name: "trims spaces", raw: "  device-123 ", want: "device-123"},
		{name: "empty", raw: "", wantErr: true},
		{name: "only spaces", raw: "   ", wantErr: true},
		{name: "too long", raw: strings.Repeat("a", MaxLength+1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrMissing)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
