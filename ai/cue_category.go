package ai

import "fmt"

const (
	CueCountFewMin  = 1
	CueCountFewMax  = 8
	CueCountSomeMin = 9
	CueCountSomeMax = 14
	CueCountManyMin = 15
)

func CueCategoryFromCount(count int) (CueCategory, error) {
	switch {
	case count >= CueCountFewMin && count <= CueCountFewMax:
		return CueCategoryFew, nil
	case count >= CueCountSomeMin && count <= CueCountSomeMax:
		return CueCategorySome, nil
	case count >= CueCountManyMin:
		return CueCategoryMany, nil
	default:
		return "", fmt.Errorf("invalid cue count: %d", count)
	}
}
