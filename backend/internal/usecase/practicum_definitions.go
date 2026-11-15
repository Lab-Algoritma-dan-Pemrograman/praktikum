package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func (s *PracticumService) CreateSession(ctx context.Context, p *entity.Principal, c PracticumCommand, in PracticumSessionInput) (entity.PracticumSession, error) {
	if !validPracticumTitle(in.Title) || in.Sequence < 1 || in.Sequence > 10000 || !validPracticumScope(in.Scope) || in.Scope.SessionID != "" {
		return entity.PracticumSession{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.session.create", "", in, func(tx repository.PracticumTx, replay string) (out entity.PracticumSession, id string, scope entity.Scope, err error) {
		scope = in.Scope
		if replay != "" {
			out, err = practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, replay, false)
			if err == nil {
				err = practicumRequire(p, practicumManage, out.Scope)
			}
			return out, replay, out.Scope, err
		}
		if err = practicumRequire(p, practicumManage, scope); err != nil {
			return
		}
		id = practicumID("session")
		scope.SessionID = id
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		out = entity.PracticumSession{ID: id, Title: in.Title, Sequence: in.Sequence, Scope: scope, Active: true, Version: 1, CreatedAt: now, UpdatedAt: now}
		err = practicumPut(tx, repository.PracticumSessions, id, "", 0, scope, "", "", nil, out)
		return
	})
}
func (s *PracticumService) UpdateSession(ctx context.Context, p *entity.Principal, c PracticumCommand, id string, in PracticumSessionUpdate) (entity.PracticumSession, error) {
	if !validPracticumTitle(in.Title) || in.Sequence < 1 || in.Sequence > 10000 || in.Version < 1 {
		return entity.PracticumSession{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.session.update", id, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumSession, target string, scope entity.Scope, err error) {
		out, err = practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, id, true)
		if err != nil {
			return
		}
		target = id
		scope = out.Scope
		if err = practicumRequire(p, practicumManage, scope); err != nil || replay != "" {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		out.Title = in.Title
		out.Sequence = in.Sequence
		out.Active = in.Active
		out.Version++
		out.UpdatedAt, err = tx.Now()
		if err == nil {
			err = practicumPut(tx, repository.PracticumSessions, id, "", 0, scope, "", "", nil, out)
		}
		return
	})
}
func (s *PracticumService) CreateCourse(ctx context.Context, p *entity.Principal, c PracticumCommand, in PracticumCourseInput) (entity.PracticumCourse, error) {
	if !validPracticumTitle(in.Title) || !validPracticumCourse(in.Kind, in.DurationMinutes) {
		return entity.PracticumCourse{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.course.create", in.SessionID, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumCourse, id string, scope entity.Scope, err error) {
		session, e := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, in.SessionID, true)
		if e != nil {
			err = e
			return
		}
		scope = session.Scope
		if err = practicumRequire(p, practicumManage, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = practicumRead[entity.PracticumCourse](tx, repository.PracticumCourses, replay, false)
			return out, replay, scope, err
		}
		id = practicumID("course")
		out = entity.PracticumCourse{ID: id, SessionID: in.SessionID, Title: in.Title, Kind: in.Kind, DurationMinutes: in.DurationMinutes, TokenRequired: in.TokenRequired, Version: 1}
		err = practicumPut(tx, repository.PracticumCourses, id, in.SessionID, 0, scope, "", in.SessionID+":"+in.Kind, nil, out)
		return
	})
}
func (s *PracticumService) UpdateCourse(ctx context.Context, p *entity.Principal, c PracticumCommand, id string, in PracticumCourseUpdate) (entity.PracticumCourse, error) {
	if !validPracticumTitle(in.Title) || in.DurationMinutes < 1 || in.DurationMinutes > 180 || in.Version < 1 {
		return entity.PracticumCourse{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.course.update", id, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumCourse, target string, scope entity.Scope, err error) {
		out, err = practicumRead[entity.PracticumCourse](tx, repository.PracticumCourses, id, true)
		if err != nil {
			return
		}
		session, e := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, out.SessionID, false)
		if e != nil {
			err = e
			return
		}
		target = id
		scope = session.Scope
		if err = practicumRequire(p, practicumManage, scope); err != nil || replay != "" {
			return
		}
		if out.Version != in.Version {
			err = repository.ErrConflict
			return
		}
		out.Title = in.Title
		out.DurationMinutes = in.DurationMinutes
		out.TokenRequired = in.TokenRequired
		out.Version++
		err = practicumPut(tx, repository.PracticumCourses, id, out.SessionID, 0, scope, "", out.SessionID+":"+out.Kind, nil, out)
		return
	})
}
func (s *PracticumService) SaveQuestion(ctx context.Context, p *entity.Principal, c PracticumCommand, id string, in PracticumQuestionUpdate) (entity.PracticumQuestion, error) {
	if !validPracticumQuestion(in.PracticumQuestionInput) || (id != "" && in.Version < 1) {
		return entity.PracticumQuestion{}, ErrValidation
	}
	action := "practicum.question.create"
	if id != "" {
		action = "practicum.question.update"
	}
	return practicumMutate(s, ctx, p, c, action, id, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumQuestion, target string, scope entity.Scope, err error) {
		courseID := in.CourseID
		var old entity.PracticumQuestion
		if id != "" {
			old, err = practicumRead[entity.PracticumQuestion](tx, repository.PracticumQuestions, id, true)
			if err != nil {
				return
			}
			if courseID != "" && courseID != old.CourseID {
				err = ErrValidation
				return
			}
			courseID = old.CourseID
		}
		course, e := practicumRead[entity.PracticumCourse](tx, repository.PracticumCourses, courseID, false)
		if e != nil {
			err = e
			return
		}
		session, e := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, course.SessionID, false)
		if e != nil {
			err = e
			return
		}
		scope = session.Scope
		if err = practicumRequire(p, practicumQuestions, scope); err != nil {
			return
		}
		if replay != "" {
			out, err = practicumRead[entity.PracticumQuestion](tx, repository.PracticumQuestions, replay, false)
			return out, replay, scope, err
		}
		target = id
		version := int64(1)
		if id == "" {
			target = practicumID("question")
		} else {
			if old.Version != in.Version {
				err = repository.ErrConflict
				return
			}
			version = old.Version + 1
		}
		out = entity.PracticumQuestion{ID: target, CourseID: courseID, Type: in.Type, Difficulty: in.Difficulty, Category: in.Category, Prompt: in.Prompt, Points: in.Points, StarterCode: in.StarterCode, Language: in.Language, ReferenceAnswer: in.ReferenceAnswer, Rubric: in.Rubric, Version: version}
		err = practicumPut(tx, repository.PracticumQuestions, target, courseID, 0, scope, "", "", nil, out)
		return
	})
}
func (s *PracticumService) Questions(ctx context.Context, p *entity.Principal, courseID string, page PracticumPage) ([]entity.PracticumQuestion, error) {
	out := []entity.PracticumQuestion{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() || courseID == "" {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		course, err := practicumRead[entity.PracticumCourse](tx, repository.PracticumCourses, courseID, false)
		if err != nil {
			return err
		}
		session, err := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, course.SessionID, false)
		if err != nil {
			return err
		}
		if err = practicumRequire(p, practicumQuestions, session.Scope); err != nil {
			return err
		}
		rows, err := tx.List(repository.PracticumQuestions, repository.PracticumFilter{ParentID: courseID, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, row := range rows {
			var q entity.PracticumQuestion
			if err = json.Unmarshal(row.Data, &q); err != nil {
				return err
			}
			out = append(out, q)
		}
		return nil
	})
	return out, err
}
func (s *PracticumService) Activate(ctx context.Context, p *entity.Principal, c PracticumCommand, sessionID string, in PracticumActivationInput) (entity.PracticumActivation, error) {
	if !validPracticumScope(in.Scope) || in.Scope.ClassID == "" || in.Scope.SessionID != sessionID || in.Scope.Shift < 1 || (in.TimerMode != "shared" && in.TimerMode != "individual") || len(in.StudentIDs) < 1 || len(in.StudentIDs) > 500 || len(in.QuestionIDs) < 1 || len(in.QuestionIDs) > 20 || len(in.EntryApprovedIDs) > 500 {
		return entity.PracticumActivation{}, ErrValidation
	}
	return practicumMutate(s, ctx, p, c, "practicum.activation.create", sessionID, in, func(tx repository.PracticumTx, replay string) (out entity.PracticumActivation, id string, scope entity.Scope, err error) {
		scope = in.Scope
		if err = practicumRequire(p, practicumManage, scope); err != nil {
			return
		}
		session, e := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, sessionID, true)
		if e != nil {
			err = e
			return
		}
		if !session.Scope.Contains(scope) {
			err = authorization.ErrNotFound
			return
		}
		if replay != "" {
			record, e := practicumRead[entity.PracticumActivationRecord](tx, repository.PracticumActivations, replay, true)
			return practicumPublicActivation(record.Activation, true), replay, scope, e
		}
		if !session.Active {
			err = ErrPracticumClosed
			return
		}
		course, e := practicumRead[entity.PracticumCourse](tx, repository.PracticumCourses, in.CourseID, false)
		if e != nil {
			err = e
			return
		}
		if course.SessionID != sessionID {
			err = authorization.ErrNotFound
			return
		}
		students := map[int64]bool{}
		for _, studentID := range in.StudentIDs {
			if studentID < 1 || students[studentID] {
				err = ErrValidation
				return
			}
			students[studentID] = true
		}
		if roster, ok := tx.(repository.PracticumRosterTx); ok {
			eligible, e := roster.EligibleStudents(in.StudentIDs, scope)
			if e != nil {
				err = e
				return
			}
			if len(eligible) != len(students) {
				err = ErrValidation
				return
			}
		} else {
			// Only isolated memory/test adapters take this path. PostgreSQL uses
			// the transaction connection above, avoiding pool starvation.
			if s.options.IdentityRepository == nil {
				err = ErrPracticumUnavailable
				return
			}
			for _, studentID := range in.StudentIDs {
				user, e := s.options.IdentityRepository.FindUserByID(ctx, studentID)
				if e != nil || !user.Active {
					err = ErrValidation
					return
				}
				assignments, e := s.options.IdentityRepository.ListAssignments(ctx, studentID)
				if e != nil {
					err = e
					return
				}
				eligible := false
				for _, a := range assignments {
					if a.Active && a.Role == entity.RoleMahasiswa && a.Scope.Contains(scope) {
						eligible = true
					}
				}
				if !eligible {
					err = ErrValidation
					return
				}
			}
		}
		approved := map[int64]bool{}
		for _, studentID := range in.EntryApprovedIDs {
			if !students[studentID] || approved[studentID] {
				err = ErrValidation
				return
			}
			approved[studentID] = true
		}
		questions := []entity.PracticumQuestion{}
		seen := map[string]bool{}
		for _, qid := range in.QuestionIDs {
			if seen[qid] {
				err = ErrValidation
				return
			}
			seen[qid] = true
			q, e := practicumRead[entity.PracticumQuestion](tx, repository.PracticumQuestions, qid, false)
			if e != nil {
				err = e
				return
			}
			if q.CourseID != course.ID {
				err = authorization.ErrNotFound
				return
			}
			questions = append(questions, q)
		}
		snapshot, e := json.Marshal(questions)
		if e != nil || len(snapshot) > 512<<10 {
			err = ErrValidation
			return
		}
		now, e := tx.Now()
		if e != nil {
			err = e
			return
		}
		if in.ClosesAt != nil {
			v := in.ClosesAt.UTC()
			in.ClosesAt = &v
			if !v.After(now) || v.After(now.Add(366*24*time.Hour)) {
				err = ErrValidation
				return
			}
		}
		id = practicumID("activation")
		out = entity.PracticumActivation{ID: id, SessionID: sessionID, CourseID: course.ID, Scope: scope, TimerMode: in.TimerMode, IsOpen: true, ClosesAt: in.ClosesAt, QuestionCount: len(questions), Version: 1, StudentIDs: in.StudentIDs, EntryApprovedIDs: in.EntryApprovedIDs, QuestionIDs: in.QuestionIDs}
		record := entity.PracticumActivationRecord{Activation: out, Course: course, Questions: questions}
		token := ""
		if course.TokenRequired {
			token, e = security.RandomToken("")
			if e != nil {
				err = e
				return
			}
			record.TokenHash = security.Hash(token)
		}
		err = practicumSaveActivation(tx, record)
		out.AccessToken = token
		return
	})
}
func practicumSaveActivation(tx repository.PracticumTx, a entity.PracticumActivationRecord) error {
	previous, err := tx.Get(repository.PracticumActivations, a.Activation.ID)
	if errors.Is(err, repository.ErrPracticumNotFound) {
		return practicumPut(tx, repository.PracticumActivations, a.Activation.ID, a.Activation.SessionID, 0, a.Activation.Scope, "", "", a.Activation.ClosesAt, a)
	}
	if err != nil {
		return err
	}
	if previous.ParentID != a.Activation.SessionID || previous.OwnerID != 0 {
		return repository.ErrConflict
	}
	raw, err := practicumMergeActivationData(previous.Data, a)
	if err != nil {
		return err
	}
	status := previous.Status
	if status == "open" || status == "closed" {
		status = "closed"
		if a.Activation.IsOpen {
			status = "open"
		}
	}
	return tx.Put(repository.PracticumActivations, repository.PracticumRecord{ID: previous.ID, ParentID: previous.ParentID, OwnerID: previous.OwnerID, Scope: previous.Scope, Status: status, UniqueKey: previous.UniqueKey, ExpiresAt: a.Activation.ClosesAt, Data: raw})
}
func (s *PracticumService) Sessions(ctx context.Context, p *entity.Principal, admin bool, page PracticumPage) ([]entity.PracticumSession, error) {
	out := []entity.PracticumSession{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		if admin {
			rows, err := tx.List(repository.PracticumSessions, repository.PracticumFilter{Scopes: practicumScopes(p, practicumManage, practicumQuestions, practicumGrade), Overlap: true, Limit: page.Limit, Offset: page.Offset})
			if err != nil {
				return err
			}
			for _, r := range rows {
				var item entity.PracticumSession
				if err = json.Unmarshal(r.Data, &item); err != nil {
					return err
				}
				out = append(out, item)
			}
			return nil
		}
		// Eligibility is applied before pagination and deduplicated across activations.
		rows, err := tx.List(repository.PracticumActivations, repository.PracticumFilter{MemberID: p.User.ID, Scopes: practicumStudentScopes(p), Limit: 1000})
		if err != nil {
			return err
		}
		seen := map[string]bool{}
		eligible := 0
		for _, r := range rows {
			var a entity.PracticumActivationRecord
			if err = json.Unmarshal(r.Data, &a); err != nil {
				return err
			}
			id := a.Activation.SessionID
			if seen[id] {
				continue
			}
			seen[id] = true
			session, err := practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, id, false)
			if err != nil {
				return err
			}
			if !session.Active {
				continue
			}
			if eligible >= page.Offset && len(out) < page.Limit {
				out = append(out, session)
			}
			eligible++
		}
		return nil
	})
	return out, err
}
func (s *PracticumService) SessionDetail(ctx context.Context, p *entity.Principal, id string, admin bool) (entity.PracticumSessionDetail, error) {
	out := entity.PracticumSessionDetail{Courses: []entity.PracticumCourse{}, Activations: []entity.PracticumActivation{}}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		var err error
		out.Session, err = practicumRead[entity.PracticumSession](tx, repository.PracticumSessions, id, false)
		if err != nil {
			return err
		}
		filter := repository.PracticumFilter{ParentID: id, Limit: 1000}
		if admin {
			scopes := practicumScopes(p, practicumManage, practicumQuestions, practicumGrade)
			ok := false
			for _, scope := range scopes {
				if _, intersects := scope.Intersect(out.Session.Scope); intersects {
					ok = true
				}
			}
			if !ok {
				return authorization.ErrNotFound
			}
			filter.Scopes = scopes
		} else {
			if !out.Session.Active {
				return authorization.ErrNotFound
			}
			filter.MemberID = p.User.ID
			filter.Scopes = practicumStudentScopes(p)
		}
		activations, err := tx.List(repository.PracticumActivations, filter)
		if err != nil {
			return err
		}
		courseIDs := map[string]bool{}
		for _, row := range activations {
			var a entity.PracticumActivationRecord
			if err = json.Unmarshal(row.Data, &a); err != nil {
				return err
			}
			out.Activations = append(out.Activations, practicumPublicActivation(a.Activation, admin))
			courseIDs[a.Course.ID] = true
		}
		if !admin && len(out.Activations) == 0 {
			return authorization.ErrNotFound
		}
		courses, err := tx.List(repository.PracticumCourses, repository.PracticumFilter{ParentID: id, Limit: 4})
		if err != nil {
			return err
		}
		for _, row := range courses {
			var course entity.PracticumCourse
			if err = json.Unmarshal(row.Data, &course); err != nil {
				return err
			}
			if admin || courseIDs[course.ID] {
				out.Courses = append(out.Courses, course)
			}
		}
		return nil
	})
	return out, err
}

// IsPracticumNotFound is useful to adapters without exposing storage errors.
func IsPracticumNotFound(err error) bool { return errors.Is(err, repository.ErrPracticumNotFound) }
