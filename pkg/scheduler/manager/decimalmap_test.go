package manager

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

func TestParse(t *testing.T) {

	testcases := []struct {
		name        string
		src         int64
		startBit    int
		expected    sets.Int
		expectError bool
	}{
		{
			name:        "1.empty src 0",
			src:         0,
			startBit:    1,
			expected:    sets.NewInt(),
			expectError: false,
		},
		{
			name:        "2.invalid src 123",
			src:         123,
			startBit:    1,
			expected:    sets.NewInt(),
			expectError: true,
		},
		{
			name:        "3. src nomal",
			src:         int64(1101),
			startBit:    1,
			expected:    sets.NewInt(1, 3, 4),
			expectError: false,
		},
		{
			name:        "4. startBit is 0",
			src:         int64(1101),
			startBit:    0,
			expected:    sets.NewInt(0, 2, 3),
			expectError: false,
		},
	}

	for _, tc := range testcases {
		as, err := NewDecimalMap(tc.src, tc.startBit).Parse()
		if err != nil {
			if !tc.expectError {
				t.Fatalf("expect error not equal, casename %s, actual %v, expect %v", tc.name, err, tc.expectError)
			}
		} else {
			if !as.Equal(tc.expected) {
				t.Fatalf("expect details not equal, casename %s, actual %v, expect %v", tc.name, as, tc.expected)
			}
		}
	}
}

func TestBuildDecimalMap(t *testing.T) {

	testcases := []struct {
		name     string
		src      sets.Int
		startBit int
		expected int64
	}{
		{
			name:     "1.empty src 0",
			src:      sets.NewInt(),
			startBit: 1,
			expected: 0,
		},
		{
			name:     "3. src nomal",
			src:      sets.NewInt(1, 3, 4),
			startBit: 1,
			expected: int64(1101),
		},
		{
			name:     "4. startBit is 0",
			src:      sets.NewInt(0, 2, 3),
			startBit: 0,
			expected: int64(1101),
		},
	}

	for _, tc := range testcases {
		as := BuildDecimalMap(tc.src, tc.startBit)
		if as != tc.expected {
			t.Fatalf("expect details not equal, casename %s, actual %v, expect %v", tc.name, as, tc.expected)
		}
	}
}
