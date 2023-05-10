package manager

import (
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {

	testcases := []struct {
		name        string
		input       []int
		expected    []int
		maxPriority int
	}{
		{
			name:        "1.normal ",
			input:       []int{1, 2, 3},
			expected:    []int{3, 6, 10},
			maxPriority: 10,
		},
		{
			name:        "2. all zero",
			input:       []int{0, 0, 0},
			expected:    []int{0, 0, 0},
			maxPriority: 100,
		},
		{
			name:        "2. maxPriority 100",
			input:       []int{1, 2, 3},
			expected:    []int{33, 66, 100},
			maxPriority: 100,
		},
	}

	for _, tc := range testcases {
		Normalize(tc.maxPriority, tc.input)
		if !reflect.DeepEqual(tc.input, tc.expected) {
			t.Errorf("test case %s, is not same, actual %v expect %v", tc.name, tc.input, tc.expected)
		}
	}
}
