package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
)

// A successful complete evaluation is a stable snapshot for an identical
// email and campaign context within this GoPhish process. Neither the input
// nor the result is written to disk. Errors never enter the cache.
const evaluationCacheEntries = 512

type evaluationCache struct {
	mu       sync.Mutex
	values   map[[32]byte][]byte
	order    [][32]byte
	inflight map[[32]byte]chan struct{}
}

func newEvaluationCache() *evaluationCache {
	return &evaluationCache{
		values:   make(map[[32]byte][]byte),
		inflight: make(map[[32]byte]chan struct{}),
	}
}

func (c *evaluationCache) Do(
	ctx context.Context,
	input EvaluationInput,
	fresh func() (EmailEvaluation, error),
) (EmailEvaluation, error) {
	key, err := evaluationCacheKey(input)
	if err != nil {
		return EmailEvaluation{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return EmailEvaluation{}, err
		}
		c.mu.Lock()
		// A refresh in progress takes precedence over an older snapshot.
		if pending, ok := c.inflight[key]; ok {
			c.mu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return EmailEvaluation{}, ctx.Err()
			}
		}
		if snapshot, ok := c.values[key]; ok {
			c.mu.Unlock()
			return decodeEvaluation(snapshot)
		}
		pending := make(chan struct{})
		c.inflight[key] = pending
		c.mu.Unlock()

		result, err := fresh()
		c.finish(key, pending, result, &err)
		return result, err
	}
}

// Refresh always calls the evaluator. On success, subsequent automatic
// evaluations reuse the new snapshot; on failure, the previous one survives.
func (c *evaluationCache) Refresh(ctx context.Context, input EvaluationInput,
	fresh func() (EmailEvaluation, error)) (EmailEvaluation, error) {
	key, err := evaluationCacheKey(input)
	if err != nil {
		return EmailEvaluation{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return EmailEvaluation{}, err
		}
		c.mu.Lock()
		if pending, ok := c.inflight[key]; ok {
			c.mu.Unlock()
			select {
			case <-pending:
				continue
			case <-ctx.Done():
				return EmailEvaluation{}, ctx.Err()
			}
		}
		pending := make(chan struct{})
		c.inflight[key] = pending
		c.mu.Unlock()

		result, err := fresh()
		c.finish(key, pending, result, &err)
		return result, err
	}
}

func evaluationCacheKey(input EvaluationInput) ([32]byte, error) {
	// The requested target and generator instructions affect how an email is
	// written, but neither enters either evaluator prompt or cue rule. Keep
	// the evaluation snapshot when only the desired difficulty changes.
	keyInput := input
	keyInput.GenerationContext.TargetDifficulty = ""
	keyInput.GenerationContext.AdditionalInstructions = ""
	request, err := json.Marshal(keyInput)
	if err != nil {
		return [32]byte{}, fmt.Errorf("marshal evaluation input: %w", err)
	}
	return sha256.Sum256(request), nil
}

func decodeEvaluation(snapshot []byte) (EmailEvaluation, error) {
	var result EmailEvaluation
	if err := json.Unmarshal(snapshot, &result); err != nil {
		return EmailEvaluation{}, fmt.Errorf("read cached evaluation: %w", err)
	}
	return result, nil
}

func (c *evaluationCache) finish(key [32]byte, pending chan struct{}, result EmailEvaluation, resultErr *error) {
	var snapshot []byte
	if *resultErr == nil {
		var err error
		snapshot, err = json.Marshal(result)
		if err != nil {
			*resultErr = fmt.Errorf("marshal evaluation snapshot: %w", err)
		}
	}
	c.mu.Lock()
	if *resultErr == nil {
		if _, exists := c.values[key]; exists {
			for index, existing := range c.order {
				if existing == key {
					c.order = append(c.order[:index], c.order[index+1:]...)
					break
				}
			}
		} else if len(c.order) == evaluationCacheEntries {
			delete(c.values, c.order[0])
			c.order = c.order[1:]
		}
		c.values[key] = snapshot
		c.order = append(c.order, key)
	}
	delete(c.inflight, key)
	close(pending)
	c.mu.Unlock()
}
