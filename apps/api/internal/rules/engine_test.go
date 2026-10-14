package rules

import (
    "testing"
    "time"

    "github.com/google/uuid"
)

var (
    retailID = uuid.MustParse("22222222-0000-7000-8000-000000000001")
    lendID   = uuid.MustParse("22222222-0000-7000-8000-000000000002")
    alice    = uuid.MustParse("44444444-0000-7000-8000-000000000007")
)

func facts(dept uuid.UUID, hasDept bool, hiredDaysAgo int, roles []string) UserFacts {
    f := UserFacts{
        UserID:       alice,
        Email:        "amaka.obi@harbourline.test",
        FullName:     "Amaka Obi",
        Status:       "active",
        RoleKeys:     roles,
        DepartmentID: dept,
        HasDept:      hasDept,
    }
    if hiredDaysAgo >= 0 {
        f.HiredOn = time.Now().AddDate(0, 0, -hiredDaysAgo)
        f.HasHiredOn = true
    }
    return f
}

func TestMatch_emptyConditions_matchEveryone(t *testing.T) {
    ok, err := Match(nil, facts(retailID, true, 3, []string{"learner"}), time.Now())
    if err != nil {
        t.Fatalf("match: %v", err)
    }
    if !ok {
        t.Errorf("expected match for empty conditions")
    }
}

func TestMatch_retailFirstWeek(t *testing.T) {
    raw := []byte(`{"and":[{"field":"department_id","op":"eq","value":"22222222-0000-7000-8000-000000000001"},{"field":"hired_within_days","op":"lte","value":7}]}`)
    now := time.Now()

    cases := []struct {
        name string
        u    UserFacts
        want bool
    }{
        {"retail 3 days",  facts(retailID, true, 3, []string{"learner"}), true},
        {"retail 10 days", facts(retailID, true, 10, []string{"learner"}), false},
        {"lending 3 days", facts(lendID, true, 3, []string{"learner"}), false},
        {"no dept",        facts(uuid.Nil, false, 3, []string{"learner"}), false},
        {"no hire date",   facts(retailID, true, -1, []string{"learner"}), false},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, err := Match(raw, tc.u, now)
            if err != nil {
                t.Fatalf("err: %v", err)
            }
            if got != tc.want {
                t.Errorf("got %v want %v", got, tc.want)
            }
        })
    }
}

func TestMatch_roleKey(t *testing.T) {
    raw := []byte(`{"and":[{"field":"role_key","op":"eq","value":"manager"}]}`)
    now := time.Now()

    got, _ := Match(raw, facts(retailID, true, 3, []string{"learner", "manager"}), now)
    if !got {
        t.Errorf("expected manager to match")
    }
    got, _ = Match(raw, facts(retailID, true, 3, []string{"learner"}), now)
    if got {
        t.Errorf("expected learner-only not to match")
    }
}

func TestMatch_statusEq(t *testing.T) {
    raw := []byte(`{"and":[{"field":"status","op":"eq","value":"active"}]}`)
    got, _ := Match(raw, facts(retailID, true, 3, nil), time.Now())
    if !got {
        t.Errorf("expected active status to match")
    }
}

func TestMatch_unknownFieldReturnsError(t *testing.T) {
    raw := []byte(`{"and":[{"field":"nonsense","op":"eq","value":"x"}]}`)
    _, err := Match(raw, facts(retailID, true, 3, nil), time.Now())
    if err == nil {
        t.Fatalf("expected error for unknown field")
    }
}