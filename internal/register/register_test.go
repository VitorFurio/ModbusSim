package register

import (
	"testing"
)

func TestDataTypeWordCount(t *testing.T) {
	tests := []struct {
		dt   DataType
		want uint16
	}{
		{TypeUint16, 1},
		{TypeInt16, 1},
		{TypeBool, 1},
		{TypeUint32, 2},
		{TypeInt32, 2},
		{TypeFloat32, 2},
		{"unknown", 1},
	}
	for _, tc := range tests {
		got := tc.dt.WordCount()
		if got != tc.want {
			t.Errorf("WordCount(%q) = %d, want %d", tc.dt, got, tc.want)
		}
	}
}

func TestRegisterWordAddresses(t *testing.T) {
	tests := []struct {
		name    string
		address uint16
		dt      DataType
		want    []uint16
	}{
		{"uint16 at 0", 0, TypeUint16, []uint16{0}},
		{"int16 at 5", 5, TypeInt16, []uint16{5}},
		{"bool at 10", 10, TypeBool, []uint16{10}},
		{"float32 at 0", 0, TypeFloat32, []uint16{0, 1}},
		{"uint32 at 2", 2, TypeUint32, []uint16{2, 3}},
		{"int32 at 4", 4, TypeInt32, []uint16{4, 5}},
	}
	for _, tc := range tests {
		r := Register{Address: tc.address, DataType: tc.dt}
		got := r.WordAddresses()
		if len(got) != len(tc.want) {
			t.Errorf("%s: len=%d, want %d", tc.name, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: addr[%d]=%d, want %d", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}
