package manager

import (
	"math"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/superedge/orin-device-system/pkg/scheduler/manager/topo"
)

const (
	AllocatorPolicyBinPack = "binpack"
	AllocatorPolicySpread  = "spread"

	BoardIDNotFount = -1
)

var (
	AllocatorMap = map[string]Allocator{AllocatorPolicyBinPack: &BinpackAllocator{}}
)

type Allocator interface {
	Allocate(canAlloc topo.BoardDetails, orinRequest sets.Int) *AllocatorResult
}

type AllocatorResult struct {
	score   int
	boardID int
}

type BinpackAllocator struct{}

func (ba *BinpackAllocator) Allocate(canAlloc topo.BoardDetails, orinRequest sets.Int) *AllocatorResult {
	bestFitBoardID := BoardIDNotFount
	bestFitBoardScore := math.MaxInt
	nodeScore := 100
	// 1. list all predicate board
	for boardID, orinInfo := range canAlloc {
		availOrinSet := orinInfo.OrinSet()
		if availOrinSet.IsSuperset(orinRequest) {
			// caculate fit score, lower is better
			tmpScore := availOrinSet.Len()
			if tmpScore < bestFitBoardScore {
				bestFitBoardScore = tmpScore
				bestFitBoardID = boardID
			}
		}
	}
	// 2. return the most binpack board

	if bestFitBoardID != BoardIDNotFount {
		// caculate node score
		for _, orinInfo := range canAlloc {
			nodeScore -= len(orinInfo)
		}
	} else {
		nodeScore = 0
	}
	return &AllocatorResult{score: nodeScore, boardID: bestFitBoardID}
}

type SpreadAllocator struct{}

// TODO implement
func (sa *SpreadAllocator) Allocate(canAlloc topo.BoardDetails, orinRequest sets.Int) *AllocatorResult {
	panic("unimplement")
}
