package provider

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestGetOrinClasses(t *testing.T) {

	testcases := []struct {
		name     string
		input    *OrinFileDevice
		expected map[int]sets.Int
	}{
		{
			name:     "1.empty device",
			input:    &OrinFileDevice{BoardDevices: []*Device{{OrinSocs: []*OrinSoc{}}}},
			expected: map[int]sets.Int{},
		},
		{
			name:     "2.normal device",
			input:    &OrinFileDevice{BoardDevices: []*Device{{ID: 0, OrinSocs: []*OrinSoc{{ID: 1}, {ID: 2}}}, {ID: 1, OrinSocs: []*OrinSoc{{ID: 1}, {ID: 2}, {ID: 3}}}}},
			expected: map[int]sets.Int{1: sets.NewInt(0, 1), 2: sets.NewInt(0, 1), 3: sets.NewInt(1)},
		},
	}
	for _, tc := range testcases {
		fop := FileDeviceProvider{FilePath: "", FileDevice: tc.input}
		actual := fop.GetOrinClasses()
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("test case %s, is not same, expect %v, actual %v", tc.name, tc.expected, actual)
		}
	}
}
