package scorm

import "testing"

func TestIsWritable(t *testing.T) {
    cases := []struct {
        v    Version
        key  string
        want bool
    }{
        {V12, "cmi.core.lesson_status", true},
        {V12, "cmi.core.student_id", false},
        {V12, "cmi.core.score.raw", true},
        {V12, "cmi.suspend_data", true},
        {V12, "cmi.objectives.0.id", true},
        {V12, "cmi.objectives.0.bogus", false},
        {V2004, "cmi.completion_status", true},
        {V2004, "cmi.learner_id", false},
        {V2004, "cmi.score.scaled", true},
        {V2004, "cmi.interactions.2.result", true},
        {V2004, "cmi.interactions.2.nonsense", false},
        {V12, "cmi.core.", false},
        {V12, "", false},
    }
    for _, c := range cases {
        if got := IsWritable(c.v, c.key); got != c.want {
            t.Errorf("IsWritable(%s, %q) = %v, want %v", c.v, c.key, got, c.want)
        }
    }
}

func TestParseSessionTime12(t *testing.T) {
    cases := map[string]int{
        "00:00:00":   0,
        "00:01:30":   90,
        "01:00:00":   3600,
        "00:00:05.5": 5,
        "02:30:15":   9015,
    }
    for in, want := range cases {
        got, err := ParseSessionTime(V12, in)
        if err != nil {
            t.Fatalf("ParseSessionTime(1.2, %q) error: %v", in, err)
        }
        if got != want {
            t.Errorf("ParseSessionTime(1.2, %q) = %d, want %d", in, got, want)
        }
    }
}

func TestParseSessionTime2004(t *testing.T) {
    cases := map[string]int{
        "PT0S":       0,
        "PT1M30S":    90,
        "PT1H":       3600,
        "PT1H15M20S": 4520,
        "PT20.5S":    21,
        "P1DT2H":     93600,
    }
    for in, want := range cases {
        got, err := ParseSessionTime(V2004, in)
        if err != nil {
            t.Fatalf("ParseSessionTime(2004, %q) error: %v", in, err)
        }
        if got != want {
            t.Errorf("ParseSessionTime(2004, %q) = %d, want %d", in, got, want)
        }
    }
}

func TestDerive12Passed(t *testing.T) {
    cmi := map[string]string{
        "cmi.core.lesson_status": "passed",
        "cmi.core.score.raw":     "85",
        "cmi.core.score.min":     "0",
        "cmi.core.score.max":     "100",
        "cmi.core.session_time":  "00:12:34",
        "cmi.suspend_data":       "abcdef",
    }
    d := Derive(V12, cmi)
    if d.CompletionStatus != "completed" || d.SuccessStatus != "passed" {
        t.Fatalf("bad status: %+v", d)
    }
    if d.ScoreRaw == nil || *d.ScoreRaw != 85 {
        t.Fatalf("bad score: %+v", d.ScoreRaw)
    }
    if d.SessionTimeS != 754 {
        t.Fatalf("bad session_time_s: %d", d.SessionTimeS)
    }
    if StatusForDerived(d) != StatusPassed {
        t.Fatalf("bad status map: %s", StatusForDerived(d))
    }
}

func TestDerive2004Failed(t *testing.T) {
    cmi := map[string]string{
        "cmi.completion_status": "completed",
        "cmi.success_status":    "failed",
        "cmi.score.scaled":      "0.4",
        "cmi.session_time":      "PT1M10S",
    }
    d := Derive(V2004, cmi)
    if d.SuccessStatus != "failed" {
        t.Fatalf("bad success: %q", d.SuccessStatus)
    }
    if StatusForDerived(d) != StatusFailed {
        t.Fatalf("bad status map: %s", StatusForDerived(d))
    }
    if d.SessionTimeS != 70 {
        t.Fatalf("bad session: %d", d.SessionTimeS)
    }
}

func TestDetectVersion(t *testing.T) {
    if DetectVersion(map[string]string{"cmi.core.lesson_status": "incomplete"}) != V12 {
        t.Fatal("expected V12")
    }
    if DetectVersion(map[string]string{"cmi.learner_id": "x"}) != V2004 {
        t.Fatal("expected V2004")
    }
    if DetectVersion(nil) != V12 {
        t.Fatal("empty should default V12")
    }
}