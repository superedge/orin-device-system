package manager

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/superedge/orin-device-system/pkg/scheduler/manager/topo"
)

func TestAllocate(t *testing.T) {

	alloc := BinpackAllocator{}

	case2Alloc := topo.NewBoardDetails()
	case2Alloc[0] = topo.NewOrinDetails().Add(0, 0).Add(0, 1)
	case2Alloc[1] = topo.NewOrinDetails().Add(1, 0).Add(1, 1)
	case2Alloc[2] = topo.NewOrinDetails().Add(2, 0).Add(2, 1)

	case3Alloc := topo.NewBoardDetails()
	case3Alloc[0] = topo.NewOrinDetails().Add(0, 0).Add(0, 1)
	case3Alloc[1] = topo.NewOrinDetails().Add(1, 1).Add(1, 2)
	case3Alloc[2] = topo.NewOrinDetails().Add(2, 1).Add(2, 2)

	case4Alloc := topo.NewBoardDetails()
	case4Alloc[0] = topo.NewOrinDetails().Add(0, 0).Add(0, 1).Add(0, 2)
	case4Alloc[1] = topo.NewOrinDetails().Add(1, 0).Add(1, 1)
	case4Alloc[2] = topo.NewOrinDetails().Add(2, 0).Add(2, 1).Add(2, 2)

	testcases := []struct {
		name     string
		canAlloc topo.BoardDetails
		request  sets.Int
		expected AllocatorResult
	}{
		{
			name:     "1.empty available resouce",
			canAlloc: topo.NewBoardDetails(),
			request:  sets.NewInt(),
			expected: AllocatorResult{score: 0, boardID: BoardIDNotFount},
		},
		{
			name:     "2.resouce not enough",
			canAlloc: case2Alloc,
			request:  sets.NewInt(1, 2),
			expected: AllocatorResult{score: 0, boardID: BoardIDNotFount},
		},
		{
			name:     "3.resouce enough  only 1 board can allocate",
			canAlloc: case3Alloc,
			request:  sets.NewInt(0, 1),
			expected: AllocatorResult{score: 94, boardID: 0},
		},
		{
			name:     "4.resouce enough  3 board can allocate, allocate a most binpack one",
			canAlloc: case4Alloc,
			request:  sets.NewInt(0, 1),
			expected: AllocatorResult{score: 92, boardID: 1},
		},
	}

	for _, tc := range testcases {
		ar := alloc.Allocate(tc.canAlloc, tc.request)
		if tc.expected.boardID != ar.boardID || tc.expected.score != ar.score {
			t.Errorf("test case %s, is not same, expect boardID=%v,score=%v, actual boardID=%v,score=%v", tc.name, tc.expected.boardID, tc.expected.score, ar.boardID, ar.score)
		}
	}
}
