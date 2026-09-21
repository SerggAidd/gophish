package ai

import "fmt"

func CueCategoryFromCount(count int) (CueCategory, error) {
	switch {
	case count >= 1 && count <= 8:
		return CueCategoryFew, nil
	case count >= 9 && count <= 14:
		return CueCategorySome, nil
	case count >= 15:
		return CueCategoryMany, nil
	default:
		return "", fmt.Errorf("invalid cue count: %d", count)
	}
}
