package gamification

import "testing"

func TestCumulativeXP_matchesPlan(t *testing.T) {
cases := map[int]int64{
1:  0,
2:  282,
5:  1118,
10: 3162,
20: 8944,
}
for level, want := range cases {
got := CumulativeXP(level)
if got != want {
t.Errorf("CumulativeXP(%d) = %d, want %d", level, got, want)
}
}
}

func TestCumulativeXP_zeroForInvalidLevel(t *testing.T) {
if CumulativeXP(0) != 0 {
t.Errorf("CumulativeXP(0) = %d, want 0", CumulativeXP(0))
}
if CumulativeXP(-5) != 0 {
t.Errorf("CumulativeXP(-5) = %d, want 0", CumulativeXP(-5))
}
}

func TestLevelForXP_boundaries(t *testing.T) {
cases := []struct {
total int64
want  int
}{
{0, 1},
{95, 1},
{281, 1},
{282, 2},
{518, 2},
{519, 3},
{799, 3},
{800, 4},
{1117, 4},
{1118, 5},
{3161, 9},
{3162, 10},
}
for _, tc := range cases {
got := LevelForXP(tc.total)
if got != tc.want {
t.Errorf("LevelForXP(%d) = %d, want %d", tc.total, got, tc.want)
}
}
}

func TestProgressFor_at95XP(t *testing.T) {
// The exact case from the smoke test: 95 XP should be level 1,
// 95 into level, 282 to level 2, 33%.
p := ProgressFor(95)
if p.Level != 1 {
t.Errorf("level = %d, want 1", p.Level)
}
if p.XPIntoLevel != 95 {
t.Errorf("xp_into_level = %d, want 95", p.XPIntoLevel)
}
if p.XPForNextLevel != 282 {
t.Errorf("xp_for_next = %d, want 282", p.XPForNextLevel)
}
if p.ProgressPct != 33 {
t.Errorf("progress_pct = %d, want 33", p.ProgressPct)
}
}

func TestProgressFor_at500XP(t *testing.T) {
p := ProgressFor(500)
if p.Level != 2 {
t.Errorf("level = %d, want 2", p.Level)
}
if p.XPIntoLevel != 218 {
t.Errorf("xp_into_level = %d, want 218", p.XPIntoLevel)
}
if p.XPForNextLevel != 519 {
t.Errorf("xp_for_next = %d, want 519", p.XPForNextLevel)
}
if p.ProgressPct < 90 || p.ProgressPct > 93 {
t.Errorf("progress_pct = %d, want ~91", p.ProgressPct)
}
}

func TestProgressFor_freshUser(t *testing.T) {
p := ProgressFor(0)
if p.Level != 1 {
t.Errorf("level = %d, want 1", p.Level)
}
if p.XPIntoLevel != 0 {
t.Errorf("xp_into_level = %d, want 0", p.XPIntoLevel)
}
if p.XPForNextLevel != 282 {
t.Errorf("xp_for_next = %d, want 282", p.XPForNextLevel)
}
if p.ProgressPct != 0 {
t.Errorf("progress_pct = %d, want 0", p.ProgressPct)
}
}