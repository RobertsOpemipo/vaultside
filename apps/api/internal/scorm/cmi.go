// Package scorm implements the SCORM 1.2 and SCORM 2004 run-time: the CMI
// data model, key validation, attempt state and commit/finish semantics.
package scorm

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"
)

// Version identifies the SCORM data model in use.
type Version string

const (
    V12   Version = "1.2"
    V2004 Version = "2004"
)

// Statuses written to scorm_attempts.status.
const (
    StatusInProgress = "in_progress"
    StatusCompleted  = "completed"
    StatusPassed     = "passed"
    StatusFailed     = "failed"
    StatusAbandoned  = "abandoned"
)

// indexedKey matches slot keys such as cmi.objectives.0.id or
// cmi.interactions.3.result.
var indexedKey = regexp.MustCompile(`^cmi\.(objectives|interactions)\.(\d+)\.([A-Za-z_]+(?:\.[A-Za-z_0-9]+)*)$`)

// readOnly12 lists SCORM 1.2 keys the content may read but not write.
var readOnly12 = map[string]bool{
    "cmi.core.student_id":                true,
    "cmi.core.student_name":              true,
    "cmi.core.credit":                    true,
    "cmi.core.entry":                     true,
    "cmi.core.total_time":                true,
    "cmi.core.lesson_mode":               true,
    "cmi.launch_data":                    true,
    "cmi.comments_from_lms":              true,
    "cmi.student_data.mastery_score":     true,
    "cmi.student_data.max_time_allowed":  true,
    "cmi.student_data.time_limit_action": true,
}

// writable12 lists SCORM 1.2 keys the content may write.
var writable12 = map[string]bool{
    "cmi.core.lesson_location":        true,
    "cmi.core.lesson_status":          true,
    "cmi.core.score.raw":              true,
    "cmi.core.score.min":              true,
    "cmi.core.score.max":              true,
    "cmi.core.exit":                   true,
    "cmi.core.session_time":           true,
    "cmi.suspend_data":                true,
    "cmi.comments":                    true,
    "cmi.student_preference.audio":    true,
    "cmi.student_preference.language": true,
    "cmi.student_preference.speed":    true,
    "cmi.student_preference.text":     true,
}

// readOnly2004 lists SCORM 2004 read-only keys.
var readOnly2004 = map[string]bool{
    "cmi.learner_id":               true,
    "cmi.learner_name":             true,
    "cmi.credit":                   true,
    "cmi.entry":                    true,
    "cmi.mode":                     true,
    "cmi.total_time":               true,
    "cmi.launch_data":              true,
    "cmi.scaled_passing_score":     true,
    "cmi.completion_threshold":     true,
    "cmi.comments_from_lms._count": true,
    "cmi.objectives._count":        true,
    "cmi.interactions._count":      true,
}

// writable2004 lists SCORM 2004 writable keys.
var writable2004 = map[string]bool{
    "cmi.completion_status":                   true,
    "cmi.success_status":                      true,
    "cmi.score.raw":                           true,
    "cmi.score.min":                           true,
    "cmi.score.max":                           true,
    "cmi.score.scaled":                        true,
    "cmi.progress_measure":                    true,
    "cmi.location":                            true,
    "cmi.suspend_data":                        true,
    "cmi.exit":                                true,
    "cmi.session_time":                        true,
    "cmi.comments":                            true,
    "cmi.learner_preference.audio_level":      true,
    "cmi.learner_preference.language":         true,
    "cmi.learner_preference.delivery_speed":   true,
    "cmi.learner_preference.audio_captioning": true,
}

// indexedSubfields12 lists writable subfields on cmi.objectives.N.* and
// cmi.interactions.N.* for SCORM 1.2.
var indexedSubfields12 = map[string]map[string]bool{
    "objectives": {
        "id": true, "score.raw": true, "score.min": true,
        "score.max": true, "status": true,
    },
    "interactions": {
        "id": true, "type": true, "objectives.0.id": true,
        "timestamp": true, "correct_responses.0.pattern": true,
        "weighting": true, "learner_response": true,
        "result": true, "latency": true, "description": true,
    },
}

// indexedSubfields2004 lists writable subfields for SCORM 2004.
var indexedSubfields2004 = map[string]map[string]bool{
    "objectives": {
        "id": true, "score.scaled": true, "score.raw": true,
        "score.min": true, "score.max": true, "success_status": true,
        "completion_status": true, "progress_measure": true,
        "description": true,
    },
    "interactions": {
        "id": true, "type": true, "objectives.0.id": true,
        "timestamp": true, "correct_responses.0.pattern": true,
        "weighting": true, "learner_response": true,
        "result": true, "latency": true, "description": true,
    },
}

// IsWritable reports whether the content may write the given CMI key.
func IsWritable(v Version, key string) bool {
    if v == V2004 {
        if writable2004[key] {
            return true
        }
    } else if writable12[key] {
        return true
    }
    m := indexedKey.FindStringSubmatch(key)
    if m == nil {
        return false
    }
    var subs map[string]map[string]bool
    if v == V2004 {
        subs = indexedSubfields2004
    } else {
        subs = indexedSubfields12
    }
    allowed, ok := subs[m[1]]
    if !ok {
        return false
    }
    return allowed[m[3]]
}

// IsReadable reports whether the content may read the key. Anything writable
// is readable; a small set of learner and course fields is readable only.
func IsReadable(v Version, key string) bool {
    if IsWritable(v, key) {
        return true
    }
    if v == V2004 {
        return readOnly2004[key]
    }
    return readOnly12[key]
}

// InitialCMI returns the CMI dictionary a fresh launch starts with.
func InitialCMI(v Version, learnerID, learnerName string, resume bool) map[string]string {
    entry := "ab-initio"
    if resume {
        entry = "resume"
    }
    if v == V2004 {
        return map[string]string{
            "cmi.learner_id":        learnerID,
            "cmi.learner_name":      learnerName,
            "cmi.credit":            "credit",
            "cmi.mode":              "normal",
            "cmi.entry":             entry,
            "cmi.completion_status": "unknown",
            "cmi.success_status":    "unknown",
            "cmi.exit":              "",
            "cmi.location":          "",
            "cmi.suspend_data":      "",
            "cmi.total_time":        "PT0H0M0S",
        }
    }
    return map[string]string{
        "cmi.core.student_id":      learnerID,
        "cmi.core.student_name":    learnerName,
        "cmi.core.credit":          "credit",
        "cmi.core.lesson_mode":     "normal",
        "cmi.core.entry":           entry,
        "cmi.core.lesson_status":   "not attempted",
        "cmi.core.exit":            "",
        "cmi.core.lesson_location": "",
        "cmi.suspend_data":         "",
        "cmi.core.total_time":      "00:00:00",
    }
}

// DetectVersion infers the SCORM version from a CMI dictionary's key shapes.
func DetectVersion(cmi map[string]string) Version {
    for k := range cmi {
        if strings.HasPrefix(k, "cmi.core.") ||
            k == "cmi.launch_data" || k == "cmi.comments" {
            return V12
        }
        if k == "cmi.learner_id" || k == "cmi.learner_name" ||
            k == "cmi.completion_status" || k == "cmi.success_status" {
            return V2004
        }
    }
    return V12
}

// ParseSessionTime converts a SCORM time value to whole seconds.
//   SCORM 1.2: HH:MM:SS(.SS)?
//   SCORM 2004: ISO 8601 duration, e.g. PT1H15M20.5S
func ParseSessionTime(v Version, s string) (int, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return 0, nil
    }
    if v == V2004 {
        return parseISO8601Duration(s)
    }
    return parseCMITime(s)
}

func parseCMITime(s string) (int, error) {
    if i := strings.IndexByte(s, '.'); i >= 0 {
        s = s[:i]
    }
    parts := strings.Split(s, ":")
    if len(parts) != 3 {
        return 0, fmt.Errorf("scorm: bad session_time %q", s)
    }
    h, err := strconv.Atoi(parts[0])
    if err != nil {
        return 0, fmt.Errorf("scorm: bad hours: %w", err)
    }
    m, err := strconv.Atoi(parts[1])
    if err != nil {
        return 0, fmt.Errorf("scorm: bad minutes: %w", err)
    }
    sec, err := strconv.Atoi(parts[2])
    if err != nil {
        return 0, fmt.Errorf("scorm: bad seconds: %w", err)
    }
    return h*3600 + m*60 + sec, nil
}

var isoDurRe = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

func parseISO8601Duration(s string) (int, error) {
    m := isoDurRe.FindStringSubmatch(s)
    if m == nil {
        return 0, fmt.Errorf("scorm: bad session_time %q", s)
    }
    toInt := func(x string) int { n, _ := strconv.Atoi(x); return n }
    d := toInt(m[1])
    h := toInt(m[2])
    mi := toInt(m[3])
    sec := 0
    if m[4] != "" {
        f, _ := strconv.ParseFloat(m[4], 64)
        sec = int(f + 0.5)
    }
    return d*86400 + h*3600 + mi*60 + sec, nil
}

// Derived holds the typed columns extracted from a CMI dictionary for
// indexing, reporting and compliance rollup.
type Derived struct {
    ScoreRaw         *float64
    ScoreMin         *float64
    ScoreMax         *float64
    CompletionStatus string // 'completed','incomplete','not attempted','unknown'
    SuccessStatus    string // 'passed','failed','unknown'
    SuspendData      string
    SessionTimeS     int
}

// Derive extracts the typed columns from a CMI dictionary for either version.
func Derive(v Version, cmi map[string]string) Derived {
    var d Derived
    if v == V2004 {
        d.CompletionStatus = cmi["cmi.completion_status"]
        d.SuccessStatus = cmi["cmi.success_status"]
        d.SuspendData = cmi["cmi.suspend_data"]
        d.ScoreRaw = parseNum(cmi["cmi.score.raw"])
        d.ScoreMin = parseNum(cmi["cmi.score.min"])
        d.ScoreMax = parseNum(cmi["cmi.score.max"])
        if s := cmi["cmi.session_time"]; s != "" {
            if n, err := parseISO8601Duration(s); err == nil {
                d.SessionTimeS = n
            }
        }
        return d
    }
    // SCORM 1.2 -> normalise to 2004 vocabulary.
    switch cmi["cmi.core.lesson_status"] {
    case "passed":
        d.CompletionStatus = "completed"
        d.SuccessStatus = "passed"
    case "failed":
        d.CompletionStatus = "completed"
        d.SuccessStatus = "failed"
    case "completed":
        d.CompletionStatus = "completed"
    case "incomplete", "browsed", "not attempted", "":
        d.CompletionStatus = "incomplete"
    default:
        d.CompletionStatus = cmi["cmi.core.lesson_status"]
    }
    d.SuspendData = cmi["cmi.suspend_data"]
    d.ScoreRaw = parseNum(cmi["cmi.core.score.raw"])
    d.ScoreMin = parseNum(cmi["cmi.core.score.min"])
    d.ScoreMax = parseNum(cmi["cmi.core.score.max"])
    if s := cmi["cmi.core.session_time"]; s != "" {
        if n, err := parseCMITime(s); err == nil {
            d.SessionTimeS = n
        }
    }
    return d
}

func parseNum(s string) *float64 {
    if s == "" {
        return nil
    }
    f, err := strconv.ParseFloat(s, 64)
    if err != nil {
        return nil
    }
    return &f
}

// StatusForDerived maps a Derived into the scorm_attempts.status column.
func StatusForDerived(d Derived) string {
    switch d.SuccessStatus {
    case "passed":
        return StatusPassed
    case "failed":
        return StatusFailed
    }
    if d.CompletionStatus == "completed" {
        return StatusCompleted
    }
    return StatusInProgress
}