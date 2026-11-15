package usecase

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

type practicumRawObject map[string]json.RawMessage

func practicumEqualTimes(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func practicumPreserveRawField(proposed, previous practicumRawObject, key string) {
	if saved, ok := previous[key]; ok {
		proposed[key] = saved
	} else {
		delete(proposed, key)
	}
}

// Closing an imported activation or starting its shared timer may change live
// state, never its roster, question/course snapshots, or raw legacy metadata.
func practicumMergeActivationData(previous json.RawMessage, next entity.PracticumActivationRecord) (json.RawMessage, error) {
	var prior entity.PracticumActivationRecord
	if err := json.Unmarshal(previous, &prior); err != nil {
		return nil, err
	}
	before, after := prior.Activation, next.Activation
	if before.ID != after.ID || before.SessionID != after.SessionID || before.CourseID != after.CourseID || !reflect.DeepEqual(before.Scope, after.Scope) || before.TimerMode != after.TimerMode || before.QuestionCount != after.QuestionCount || !reflect.DeepEqual(before.StudentIDs, after.StudentIDs) || !reflect.DeepEqual(before.EntryApprovedIDs, after.EntryApprovedIDs) || !reflect.DeepEqual(before.QuestionIDs, after.QuestionIDs) || !practicumEqualTimes(before.ClosesAt, after.ClosesAt) || !reflect.DeepEqual(prior.Course, next.Course) || !reflect.DeepEqual(prior.Questions, next.Questions) || prior.TokenHash != next.TokenHash || !before.IsOpen && after.IsOpen {
		return nil, repository.ErrConflict
	}
	if before.StartedAt != nil && (!practicumEqualTimes(before.StartedAt, after.StartedAt) || !practicumEqualTimes(before.DeadlineAt, after.DeadlineAt)) {
		return nil, repository.ErrConflict
	}
	var old, oldActivation practicumRawObject
	if err := json.Unmarshal(previous, &old); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(old["activation"], &oldActivation); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	var proposed, proposedActivation practicumRawObject
	if err := json.Unmarshal(raw, &proposed); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(proposed["activation"], &proposedActivation); err != nil {
		return nil, err
	}
	for _, key := range []string{"id", "session_id", "course_id", "scope", "student_ids", "entry_approved_ids", "question_ids", "timer_mode", "closes_at"} {
		practicumPreserveRawField(proposedActivation, oldActivation, key)
	}
	if practicumEqualTimes(before.StartedAt, after.StartedAt) {
		practicumPreserveRawField(proposedActivation, oldActivation, "started_at")
	}
	if practicumEqualTimes(before.DeadlineAt, after.DeadlineAt) {
		practicumPreserveRawField(proposedActivation, oldActivation, "deadline_at")
	}
	for key, value := range proposedActivation {
		oldActivation[key] = value
	}
	proposed["activation"], err = json.Marshal(oldActivation)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"course", "questions", "token_hash"} {
		practicumPreserveRawField(proposed, old, key)
	}
	for key, value := range proposed {
		old[key] = value
	}
	return json.Marshal(old)
}

// Domain structs intentionally omit private legacy fields from API responses.
// Persistence must not use that omission to discard imported metadata or rewrite
// immutable JSON snapshots (including equivalent timestamps with another format).
func practicumMergeAttemptData(previous json.RawMessage, next entity.PracticumAttemptRecord) (json.RawMessage, error) {
	var prior entity.PracticumAttemptRecord
	if err := json.Unmarshal(previous, &prior); err != nil {
		return nil, err
	}
	if prior.Attempt.AttemptID != next.Attempt.AttemptID || prior.Attempt.ActivationID != next.Attempt.ActivationID || prior.Attempt.UserID != next.Attempt.UserID || prior.Attempt.CourseID != next.Attempt.CourseID || !prior.Attempt.StartedAt.Equal(next.Attempt.StartedAt) || !prior.Attempt.DeadlineAt.Equal(next.Attempt.DeadlineAt) || !reflect.DeepEqual(prior.Attempt.Questions, next.Attempt.Questions) {
		return nil, repository.ErrConflict
	}
	var old, oldAttempt practicumRawObject
	if err := json.Unmarshal(previous, &old); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(old["attempt"], &oldAttempt); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return nil, err
	}
	var proposed, proposedAttempt practicumRawObject
	if err = json.Unmarshal(raw, &proposed); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(proposed["attempt"], &proposedAttempt); err != nil {
		return nil, err
	}
	var previousAnswers, answers []practicumRawObject
	if err = json.Unmarshal(oldAttempt["answers"], &previousAnswers); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(proposedAttempt["answers"], &answers); err != nil {
		return nil, err
	}
	for i, answer := range answers {
		if i >= len(previousAnswers) || i >= len(prior.Attempt.Answers) || prior.Attempt.Answers[i].QuestionID != next.Attempt.Answers[i].QuestionID {
			continue
		}
		merged := make(practicumRawObject, len(previousAnswers[i])+len(answer))
		for key, value := range previousAnswers[i] {
			merged[key] = value
		}
		for key, value := range answer {
			merged[key] = value
		}
		if prior.Attempt.Answers[i].UpdatedAt.Equal(next.Attempt.Answers[i].UpdatedAt) {
			if saved, ok := previousAnswers[i]["updated_at"]; ok {
				merged["updated_at"] = saved
			}
		}
		// Omitempty must not resurrect an older feedback or language when a user
		// deliberately clears it. Preserve missing fields when they were missing.
		if _, ok := previousAnswers[i]["feedback"]; ok && next.Attempt.Answers[i].Feedback == "" {
			merged["feedback"] = json.RawMessage(`""`)
		}
		if _, ok := previousAnswers[i]["language"]; ok && next.Attempt.Answers[i].Language == "" {
			merged["language"] = json.RawMessage(`""`)
		}
		if _, ok := previousAnswers[i]["score"]; ok && next.Attempt.Answers[i].Score == nil {
			merged["score"] = json.RawMessage(`null`)
		}
		answers[i] = merged
	}
	answerBytes, err := json.Marshal(answers)
	if err != nil {
		return nil, err
	}
	proposedAttempt["answers"] = answerBytes
	for _, key := range []string{"questions", "started_at", "deadline_at"} {
		if saved, ok := oldAttempt[key]; ok {
			proposedAttempt[key] = saved
		} else {
			delete(proposedAttempt, key)
		}
	}
	for key, value := range proposedAttempt {
		oldAttempt[key] = value
	}
	proposed["attempt"], err = json.Marshal(oldAttempt)
	if err != nil {
		return nil, err
	}
	if prior.Attempt.Status == "in_progress" && next.Attempt.Status != "in_progress" {
		proposed["submitted_answers"] = answerBytes
	} else if saved, ok := old["submitted_answers"]; ok {
		proposed["submitted_answers"] = saved
	} else {
		delete(proposed, "submitted_answers")
	}
	for key, value := range proposed {
		old[key] = value
	}
	return json.Marshal(old)
}
