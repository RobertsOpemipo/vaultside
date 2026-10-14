// Package gamification contains the pure business rules for XP, levels,
// streaks, and badges. No database access - callers pass in the values they
// already have.
package gamification

import "math"

// CumulativeXP returns the total XP a user must accumulate to be at the given
// level. Formula from the plan: floor(100 * n^1.5) for n >= 2, and 0 for n = 1
// (a fresh user starts at level 1 with 0 XP).
//
//CumulativeXP(1)  = 0
//CumulativeXP(2)  = 282
//CumulativeXP(5)  = 1118
//CumulativeXP(10) = 3162
//CumulativeXP(20) = 8944
func CumulativeXP(level int) int64 {
if level <= 1 {
return 0
}
return int64(math.Floor(100.0 * math.Pow(float64(level), 1.5)))
}

// LevelForXP returns the level a user with this much cumulative XP has reached.
func LevelForXP(total int64) int {
level := 1
for level < 10000 {
if CumulativeXP(level+1) <= total {
level++
} else {
break
}
}
return level
}

// LevelProgress describes where a user sits within their current level.
type LevelProgress struct {
Level          int   `json:"level"`
TotalXP        int64 `json:"total_xp"`
XPIntoLevel    int64 `json:"xp_into_level"`
XPForNextLevel int64 `json:"xp_for_next_level"`
ProgressPct    int   `json:"progress_pct"`
}

// ProgressFor computes level, XP into the current level, XP required for the
// next level, and a 0-100 progress percentage.
func ProgressFor(total int64) LevelProgress {
level := LevelForXP(total)
curThreshold := CumulativeXP(level)
nextThreshold := CumulativeXP(level + 1)

intoLevel := total - curThreshold
if intoLevel < 0 {
intoLevel = 0
}
span := nextThreshold - curThreshold
if span <= 0 {
span = 1
}
pct := int(intoLevel * 100 / span)
if pct > 100 {
pct = 100
}
if pct < 0 {
pct = 0
}
return LevelProgress{
Level:          level,
TotalXP:        total,
XPIntoLevel:    intoLevel,
XPForNextLevel: nextThreshold,
ProgressPct:    pct,
}
}