package topo

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

type BoardDetails map[int]OrinDetails
type OrinDetails map[int]OrinInfo

func NewBoardDetails() BoardDetails {
	emptyBoardDetails := make(map[int]OrinDetails)
	for k := range emptyBoardDetails {
		emptyBoardDetails[k] = NewOrinDetails()
	}
	return emptyBoardDetails
}

func NewOrinDetails() OrinDetails {
	return make(map[int]OrinInfo)
}

func (d BoardDetails) BoardSet() sets.Int {
	s := sets.NewInt()
	for k := range d {
		s.Insert(k)
	}
	return s
}

func (src BoardDetails) Equal(dst BoardDetails) bool {
	srcMSet := src.BoardSet()
	dstMSet := dst.BoardSet()
	if !srcMSet.Equal(dstMSet) {
		return false
	}
	for mid, srcOs := range src {
		srcOSet := srcOs.OrinSet()
		dstOSet := dst[mid].OrinSet()
		if !srcOSet.Equal(dstOSet) {
			return false
		}
	}
	return true
}

func (src BoardDetails) DifferenceFromSuperset(sub BoardDetails) (BoardDetails, error) {
	srcMSet := src.BoardSet()
	subMSet := sub.BoardSet()
	if !srcMSet.IsSuperset(subMSet) {
		klog.V(6).InfoS("srcset %v is not superset to subset %v", srcMSet, subMSet)
		return nil, fmt.Errorf("board details not superset")
	}
	md := NewBoardDetails()
	// copy boarddetaild which not in subset
	diffMSet := srcMSet.Difference(subMSet)
	for _, m := range diffMSet.UnsortedList() {
		md.Add(m, src[m])
	}
	// interate every orindetails to caculate diff
	for subMID, subOrinDetail := range sub {
		srcOSet := src[subMID].OrinSet()
		subOSet := subOrinDetail.OrinSet()
		if !srcOSet.IsSuperset(subOSet) {
			return nil, fmt.Errorf("orin details not superset")
		}
		newOd := NewOrinDetails()
		diffOSet := srcOSet.Difference(subOSet)
		for _, diffOID := range diffOSet.UnsortedList() {
			newOd.Add(subMID, diffOID)
		}
		md.Add(subMID, newOd)
	}
	return md, nil
}
func (d BoardDetails) Add(boardID int, od OrinDetails) BoardDetails {
	if md, ok := d[boardID]; !ok {
		d[boardID] = od
	} else {
		for k, v := range od {
			md[k] = v
		}
	}
	return d
}

func (d OrinDetails) OrinSet() sets.Int {
	s := sets.NewInt()
	for k := range d {
		s.Insert(k)
	}
	return s
}

func (d OrinDetails) Add(boardID int, orinID int) OrinDetails {
	d[orinID] = OrinInfo{BoardID: boardID}
	return d
}

type OrinInfo struct {
	BoardID int
}
