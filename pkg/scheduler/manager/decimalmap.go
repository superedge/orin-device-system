package manager

import (
	"fmt"
	"math"
	"strconv"

	"k8s.io/apimachinery/pkg/util/sets"
)

type Map interface {
	Parse() (sets.Int, error)
}

type DecimalMap struct {
	startBit int
	src      int64
	srcStr   string
}

func NewDecimalMap(src int64, startBit int) *DecimalMap {
	return &DecimalMap{startBit: startBit, src: src}
}

func (dm *DecimalMap) Parse() (sets.Int, error) {
	res := sets.NewInt()
	// first int64 to string
	srcStr := strconv.FormatInt(dm.src, 10)
	dm.srcStr = srcStr
	// check string must 0 or 1
	// default is little endian
	for i := len(dm.srcStr) - 1; i >= 0; i-- {
		if dm.srcStr[i] != '0' && dm.srcStr[i] != '1' {
			return nil, fmt.Errorf("invalid decimalMap %s", dm.srcStr)
		}
		if dm.srcStr[i] == '1' {
			res.Insert(len(dm.srcStr) - 1 - i + dm.startBit)
		}
	}
	return res, nil
}

func BuildDecimalMap(set sets.Int, startBit int) int64 {
	var res int64
	for _, n := range set.UnsortedList() {
		res += int64(math.Pow10(n - startBit))
	}
	return res
}
