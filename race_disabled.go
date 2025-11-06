//go:build !race

package logs

// raceEnabled is set to false when built without -race flag
const raceEnabled = false
