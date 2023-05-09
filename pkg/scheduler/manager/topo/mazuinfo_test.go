package topo

import (
	"testing"
)

func TestDifferenceFromSuperset(t *testing.T) {

	subDetails := NewBoardDetails()
	subDetails[0] = NewOrinDetails().Add(0, 0)
	subDetails[1] = NewOrinDetails().Add(1, 0)

	case1Alloc := NewBoardDetails()
	case1Alloc[0] = NewOrinDetails().Add(0, 0).Add(0, 1)
	case1Alloc[1] = NewOrinDetails().Add(1, 0).Add(1, 1)
	case1Alloc[2] = NewOrinDetails().Add(2, 0).Add(2, 1)

	case2Alloc := NewBoardDetails()
	case2Alloc[0] = NewOrinDetails().Add(0, 0).Add(0, 2)
	case2Alloc[2] = NewOrinDetails().Add(2, 0).Add(2, 1)

	case3Alloc := NewBoardDetails()
	case3Alloc[0] = NewOrinDetails().Add(0, 0).Add(0, 1)
	case3Alloc[1] = NewOrinDetails().Add(1, 0)

	case4Alloc := NewBoardDetails()
	case4Alloc[0] = NewOrinDetails().Add(0, 0).Add(0, 1).Add(0, 2)
	case4Alloc[1] = NewOrinDetails().Add(1, 0).Add(1, 1)
	case4Alloc[2] = NewOrinDetails().Add(2, 0).Add(2, 1).Add(2, 2)

	testcases := []struct {
		name          string
		srcDetails    BoardDetails
		subDetails    BoardDetails
		expectDetails BoardDetails
		expectError   bool
	}{
		{
			name:          "1.sub is empty",
			srcDetails:    case1Alloc,
			subDetails:    NewBoardDetails(),
			expectDetails: case1Alloc,
			expectError:   false,
		},
		{
			name:          "2.sub is not superset",
			srcDetails:    case2Alloc,
			subDetails:    subDetails,
			expectDetails: case1Alloc,
			expectError:   true,
		},
		{
			name:          "3.sub is normal, and board length is same as src",
			srcDetails:    case3Alloc,
			subDetails:    subDetails,
			expectDetails: NewBoardDetails().Add(0, NewOrinDetails().Add(0, 1)).Add(1, NewOrinDetails()),
			expectError:   false,
		},
		{
			name:          "4.sub is normal, and board length is less than src",
			srcDetails:    case4Alloc,
			subDetails:    subDetails,
			expectDetails: NewBoardDetails().Add(0, NewOrinDetails().Add(0, 1).Add(0, 2)).Add(1, NewOrinDetails().Add(1, 1)).Add(2, NewOrinDetails().Add(2, 0).Add(2, 1).Add(2, 2)),
			expectError:   false,
		},
	}

	for _, tc := range testcases {
		acDetails, err := tc.srcDetails.DifferenceFromSuperset(tc.subDetails)
		if err != nil {
			if !tc.expectError {
				t.Fatalf("expect error not equal, casename %s, actual %v, expect %v", tc.name, err, tc.expectError)
			}
		} else {
			if !acDetails.Equal(tc.expectDetails) {
				t.Fatalf("expect details not equal, casename %s, actual %v, expect %v", tc.name, acDetails, tc.expectDetails)
			}
		}

	}
}
