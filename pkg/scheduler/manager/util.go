package manager

import (
	"strconv"
	"strings"
	"sync"

	"github.com/superedge/orin-device-system/pkg/common"
	v1 "k8s.io/api/core/v1"
)

const DefaultOrinStartBit = 1

func OrinIDFromResourceName(name string) (int64, error) {
	orinIDStr := strings.TrimPrefix(name, common.ExtendResouceTypeOrinPrefix)
	return strconv.ParseInt(orinIDStr, 10, 0)
}
func BoardIDFromResourceName(name string) (int64, error) {
	orinIDStr := strings.TrimPrefix(name, common.ExtendResouceTypeBoardPrefix)
	return strconv.ParseInt(orinIDStr, 10, 0)
}
func Parallelize(workers, pieces int, doWorkPiece func(i int)) {
	toProcess := make(chan int, pieces)
	for i := 0; i < pieces; i++ {
		toProcess <- i
	}
	close(toProcess)

	if pieces < workers {
		workers = pieces
	}

	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for piece := range toProcess {
				doWorkPiece(piece)
			}
		}()
	}
	wg.Wait()
}

func Normalize(maxPriority int, scores []int) {
	var maxCount int
	for i := range scores {
		if scores[i] > maxCount {
			maxCount = scores[i]
		}
	}

	if maxCount == 0 {
		return
	}

	for i := range scores {
		score := scores[i]
		score = maxPriority * score / maxCount
		scores[i] = score
	}
}

// GetUpdatedPodAnnotationSpec updates pod annotation with devId
func AddPodBindAnnotation(oldPod *v1.Pod, boardID int) *v1.Pod {
	newPod := oldPod.DeepCopy()
	if len(newPod.Labels) == 0 {
		newPod.Labels = map[string]string{}
	}
	if len(newPod.Annotations) == 0 {
		newPod.Annotations = map[string]string{}
	}
	boardIDStr := strconv.FormatInt(int64(boardID), 10)
	newPod.Annotations[common.AnnotationPodBindToBoard] = boardIDStr
	newPod.Labels[common.AnnotationPodBindToBoard] = boardIDStr

	return newPod
}

func RemovePodBindAnnotation(oldPod *v1.Pod) *v1.Pod {
	newPod := oldPod.DeepCopy()
	if len(newPod.Labels) == 0 {
		return newPod
	}
	if len(newPod.Annotations) == 0 {
		return newPod
	}
	delete(newPod.Annotations, common.AnnotationPodBindToBoard)
	delete(newPod.Labels, common.AnnotationPodBindToBoard)
	return newPod
}
