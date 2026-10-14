package gamification

import "testing"

func TestBadgeRules_firstLesson(t *testing.T) {
    rule := badgeRules["first_lesson"]
    if rule(BadgeEvent{LessonCount: 0}) {
        t.Error("first_lesson fired at 0")
    }
    if !rule(BadgeEvent{LessonCount: 1}) {
        t.Error("first_lesson did not fire at 1")
    }
    if !rule(BadgeEvent{LessonCount: 5}) {
        t.Error("first_lesson did not fire at 5")
    }
}

func TestBadgeRules_threeLessons(t *testing.T) {
    rule := badgeRules["three_lessons"]
    if rule(BadgeEvent{LessonCount: 2}) {
        t.Error("three_lessons fired at 2")
    }
    if !rule(BadgeEvent{LessonCount: 3}) {
        t.Error("three_lessons did not fire at 3")
    }
}

func TestBadgeRules_firstCourse(t *testing.T) {
    rule := badgeRules["first_course"]
    if rule(BadgeEvent{CourseCount: 0}) {
        t.Error("first_course fired at 0")
    }
    if !rule(BadgeEvent{CourseCount: 1}) {
        t.Error("first_course did not fire at 1")
    }
}

func TestBadgeRules_quizAce(t *testing.T) {
    rule := badgeRules["quiz_ace"]
    if rule(BadgeEvent{JustHadQuizPass: true, QuizScore: 80}) {
        t.Error("quiz_ace fired at 80")
    }
    if rule(BadgeEvent{JustHadQuizPass: false, QuizScore: 100}) {
        t.Error("quiz_ace fired without pass flag")
    }
    if !rule(BadgeEvent{JustHadQuizPass: true, QuizScore: 100}) {
        t.Error("quiz_ace did not fire at 100 with pass")
    }
}

func TestBadgeRules_streak7(t *testing.T) {
    rule := badgeRules["streak_7"]
    if rule(BadgeEvent{CurrentStreak: 6}) {
        t.Error("streak_7 fired at 6")
    }
    if !rule(BadgeEvent{CurrentStreak: 7}) {
        t.Error("streak_7 did not fire at 7")
    }
}

func TestBadgeRules_xpMilestones(t *testing.T) {
    r500 := badgeRules["xp_500"]
    if r500(BadgeEvent{TotalXP: 499}) {
        t.Error("xp_500 fired at 499")
    }
    if !r500(BadgeEvent{TotalXP: 500}) {
        t.Error("xp_500 did not fire at 500")
    }

    r1000 := badgeRules["xp_1000"]
    if r1000(BadgeEvent{TotalXP: 999}) {
        t.Error("xp_1000 fired at 999")
    }
    if !r1000(BadgeEvent{TotalXP: 1000}) {
        t.Error("xp_1000 did not fire at 1000")
    }
}