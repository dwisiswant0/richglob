package richglob

import (
	"path/filepath"
)

type walkState struct {
	seen  map[string]int
	depth int
}

func newWalkState() *walkState {
	return &walkState{seen: make(map[string]int)}
}

func resolveWalkKey(path string) string {
	key := filepath.Clean(path)

	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}

	return key
}

func (state *walkState) enter(path string) (string, bool) {
	return state.enterKey(resolveWalkKey(path))
}

func (state *walkState) enterKey(key string) (string, bool) {
	const maxWalkDepth = 10000

	if state.depth >= maxWalkDepth {
		return "", false
	}

	if state.seen[key] > 0 {
		return "", false
	}

	state.seen[key]++
	state.depth++

	return key, true
}

func (state *walkState) leave(key string) {
	if state.seen[key] > 0 {
		state.seen[key]--
		if state.seen[key] == 0 {
			delete(state.seen, key)
		}
	}

	if state.depth > 0 {
		state.depth--
	}
}
