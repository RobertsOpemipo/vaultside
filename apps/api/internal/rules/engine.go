// Package rules evaluates assignment-rule conditions against user facts.
// Pure functions, no database access.
package rules

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
)

type Condition struct {
    Field string      `json:"field"`
    Op    string      `json:"op"`
    Value interface{} `json:"value"`
}

type Group struct {
    And []Condition `json:"and"`
}

// UserFacts is the minimum information needed to evaluate any condition.
type UserFacts struct {
    UserID       uuid.UUID
    Email        string
    FullName     string
    DepartmentID uuid.UUID
    HasDept      bool
    ManagerID    uuid.UUID
    HasManager   bool
    HiredOn      time.Time
    HasHiredOn   bool
    Status       string
    RoleKeys     []string
}

// Match returns true if the user satisfies every clause.
// Empty "and" list matches everyone.
func Match(rawConditions []byte, u UserFacts, now time.Time) (bool, error) {
    if len(rawConditions) == 0 {
        return true, nil
    }
    var g Group
    if err := json.Unmarshal(rawConditions, &g); err != nil {
        return false, fmt.Errorf("parse conditions: %w", err)
    }
    for _, c := range g.And {
        ok, err := evalCondition(c, u, now)
        if err != nil {
            return false, err
        }
        if !ok {
            return false, nil
        }
    }
    return true, nil
}

func evalCondition(c Condition, u UserFacts, now time.Time) (bool, error) {
    switch c.Field {
    case "department_id":
        if !u.HasDept {
            return false, nil
        }
        return evalUUID(c.Op, u.DepartmentID, c.Value)
    case "manager_id":
        if !u.HasManager {
            return false, nil
        }
        return evalUUID(c.Op, u.ManagerID, c.Value)
    case "hired_within_days":
        if !u.HasHiredOn {
            return false, nil
        }
        days := int(now.Sub(u.HiredOn).Hours() / 24)
        return evalInt(c.Op, int64(days), c.Value)
    case "status":
        return evalString(c.Op, u.Status, c.Value)
    case "role_key":
        return evalRole(c.Op, u.RoleKeys, c.Value)
    default:
        return false, fmt.Errorf("unsupported field: %s", c.Field)
    }
}

func evalUUID(op string, left uuid.UUID, right interface{}) (bool, error) {
    rv, ok := right.(string)
    if !ok {
        return false, fmt.Errorf("uuid field requires string value")
    }
    ru, err := uuid.Parse(rv)
    if err != nil {
        return false, fmt.Errorf("invalid uuid value: %w", err)
    }
    switch op {
    case "eq":
        return left == ru, nil
    case "neq":
        return left != ru, nil
    default:
        return false, fmt.Errorf("op %q not valid for uuid", op)
    }
}

func evalInt(op string, left int64, right interface{}) (bool, error) {
    rv, ok := toInt64(right)
    if !ok {
        return false, fmt.Errorf("integer field requires numeric value")
    }
    switch op {
    case "eq":
        return left == rv, nil
    case "neq":
        return left != rv, nil
    case "lt":
        return left < rv, nil
    case "lte":
        return left <= rv, nil
    case "gt":
        return left > rv, nil
    case "gte":
        return left >= rv, nil
    default:
        return false, fmt.Errorf("unknown op %q", op)
    }
}

func evalString(op string, left string, right interface{}) (bool, error) {
    rv, ok := right.(string)
    if !ok {
        return false, fmt.Errorf("string field requires string value")
    }
    switch op {
    case "eq":
        return left == rv, nil
    case "neq":
        return left != rv, nil
    default:
        return false, fmt.Errorf("op %q not valid for string", op)
    }
}

func evalRole(op string, roles []string, right interface{}) (bool, error) {
    rv, ok := right.(string)
    if !ok {
        return false, fmt.Errorf("role_key requires string value")
    }
    found := false
    for _, r := range roles {
        if r == rv {
            found = true
            break
        }
    }
    switch op {
    case "eq":
        return found, nil
    case "neq":
        return !found, nil
    default:
        return false, fmt.Errorf("op %q not valid for role_key", op)
    }
}

func toInt64(v interface{}) (int64, bool) {
    switch n := v.(type) {
    case int:
        return int64(n), true
    case int32:
        return int64(n), true
    case int64:
        return n, true
    case float64:
        return int64(n), true
    default:
        return 0, false
    }
}