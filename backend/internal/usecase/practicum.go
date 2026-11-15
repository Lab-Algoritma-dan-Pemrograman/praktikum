package usecase

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/platform/security"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

var (
	ErrPracticumClosed      = errors.New("practicum attempt no longer accepts changes")
	ErrPracticumBusy        = errors.New("practicum execution concurrency limit")
	ErrPracticumUnavailable = errors.New("practicum provider unavailable")
)

const (
	practicumManage    = "practicum.session.manage"
	practicumQuestions = "question.manage"
	practicumGrade     = "practicum.grade.write"
)

type PracticumRunner interface {
	Run(context.Context, entity.RunRequest) (entity.RunResult, error)
	Languages() []entity.RunnerLanguage
	Available() bool
	Simulated() bool
}
type PracticumGrader interface {
	Grade(context.Context, entity.GradeRequest) (entity.GradeResult, error)
}
type PracticumOptions struct {
	Repository         repository.PracticumRepository
	IdentityRepository repository.IdentityRepository
	Runner             PracticumRunner
	Grader             PracticumGrader
	Rumpang            RumpangGenerator
	RunnerGlobalLimit  int
	RunnerUserLimit    int
	RunnerTimeout      time.Duration
	GradingTimeout     time.Duration
}
type PracticumService struct{ options PracticumOptions }

func NewPracticumService(o PracticumOptions) *PracticumService {
	if o.RunnerGlobalLimit <= 0 {
		o.RunnerGlobalLimit = 8
	}
	if o.RunnerUserLimit <= 0 {
		o.RunnerUserLimit = 2
	}
	if o.RunnerGlobalLimit > 100 {
		o.RunnerGlobalLimit = 100
	}
	if o.RunnerUserLimit > 10 {
		o.RunnerUserLimit = 10
	}
	if o.RunnerTimeout <= 0 || o.RunnerTimeout > 30*time.Second {
		o.RunnerTimeout = 15 * time.Second
	}
	if o.GradingTimeout <= 0 || o.GradingTimeout > 30*time.Second {
		o.GradingTimeout = 15 * time.Second
	}
	if r, ok := o.Repository.(*repository.MemoryPracticumRepository); ok {
		if identity, ok := o.IdentityRepository.(*repository.MemoryIdentityRepository); ok {
			r.BindIdentity(identity)
		}
	}
	return &PracticumService{options: o}
}

type PracticumCommand struct{ Key, RequestID, OperationID string }
type PracticumPage struct{ Limit, Offset int }

func (p PracticumPage) valid() bool {
	return p.Limit >= 1 && p.Limit <= 100 && p.Offset >= 0 && p.Offset <= 1000000
}

type PracticumSessionInput struct {
	Title    string       `json:"title"`
	Sequence int          `json:"sequence"`
	Scope    entity.Scope `json:"scope"`
}
type PracticumSessionUpdate struct {
	Version  int64  `json:"version"`
	Title    string `json:"title"`
	Sequence int    `json:"sequence"`
	Active   bool   `json:"active"`
}
type PracticumCourseInput struct {
	SessionID       string `json:"session_id"`
	Title           string `json:"title"`
	Kind            string `json:"kind"`
	DurationMinutes int    `json:"duration_minutes"`
	TokenRequired   bool   `json:"token_required"`
}
type PracticumCourseUpdate struct {
	Version         int64  `json:"version"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"duration_minutes"`
	TokenRequired   bool   `json:"token_required"`
}
type PracticumQuestionInput struct {
	CourseID        string             `json:"course_id"`
	Type            string             `json:"type"`
	Difficulty      string             `json:"difficulty"`
	Category        string             `json:"category"`
	Prompt          string             `json:"prompt"`
	Points          int                `json:"points"`
	StarterCode     string             `json:"starter_code"`
	Language        string             `json:"language"`
	ReferenceAnswer string             `json:"reference_answer"`
	Rubric          []entity.Criterion `json:"rubric"`
}
type PracticumQuestionUpdate struct {
	PracticumQuestionInput
	Version int64 `json:"version"`
}
type PracticumActivationInput struct {
	CourseID         string       `json:"course_id"`
	Scope            entity.Scope `json:"scope"`
	TimerMode        string       `json:"timer_mode"`
	StudentIDs       []int64      `json:"student_ids"`
	EntryApprovedIDs []int64      `json:"entry_approved_ids"`
	QuestionIDs      []string     `json:"question_ids"`
	ClosesAt         *time.Time   `json:"closes_at"`
}
type PracticumStartInput struct {
	ActivationID string `json:"activation_id"`
	AccessToken  string `json:"access_token"`
}
type PracticumAnswerInput struct {
	QuestionID string `json:"question_id"`
	Value      string `json:"value"`
	Language   string `json:"language"`
}
type PracticumSaveInput struct {
	Revision int64                  `json:"revision"`
	Answers  []PracticumAnswerInput `json:"answers"`
}
type PracticumManualInput struct {
	GradeRevision int64                         `json:"grade_revision"`
	Grades        []entity.PracticumManualGrade `json:"grades"`
	Publish       bool                          `json:"publish"`
	Reason        string                        `json:"reason"`
	// Keaktifan: optional participation score 0..100, stored per attempt.
	Keaktifan *int `json:"keaktifan,omitempty"`
}
type PracticumRunInput struct {
	QuestionID string `json:"question_id"`
	Source     string `json:"source"`
	Language   string `json:"language"`
	Version    string `json:"version"`
	Stdin      string `json:"stdin"`
}
type practicumOperation struct {
	PayloadHash string `json:"payload_hash"`
	TargetID    string `json:"target_id"`
}

var practicumKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func practicumID(prefix string) string {
	v, err := security.RandomToken("")
	if err != nil {
		panic(err)
	}
	return prefix + "-" + v
}
func practicumHash(v any) string { data, _ := json.Marshal(v); return security.Hash(string(data)) }
func practicumAuthenticated(p *entity.Principal) error {
	if p == nil {
		return authorization.ErrUnauthenticated
	}
	if !p.User.Active {
		return authorization.ErrForbidden
	}
	return nil
}
func practicumAllowed(p *entity.Principal, permission string, scope entity.Scope) bool {
	return (authorization.Service{}).Check(p, authorization.Requirement{Permission: permission, Scope: scope, ResourceExists: true}) == nil
}
func practicumRequire(p *entity.Principal, permission string, scope entity.Scope) error {
	return (authorization.Service{}).Check(p, authorization.Requirement{Permission: permission, Scope: scope, ResourceExists: true, HideNotFound: true})
}
func practicumScopes(p *entity.Principal, permissions ...string) []entity.Scope {
	out := []entity.Scope{}
	if practicumAuthenticated(p) != nil {
		return out
	}
	for _, g := range p.Grants {
		for _, permission := range permissions {
			if g.Permission == permission && practicumAllowed(p, permission, g.Scope) {
				out = append(out, g.Scope)
			}
		}
	}
	return out
}
func practicumStudentScopes(p *entity.Principal) []entity.Scope {
	out := []entity.Scope{}
	if practicumAuthenticated(p) != nil {
		return out
	}
	for _, a := range p.Assignments {
		if a.Active && a.Role == entity.RoleMahasiswa {
			out = append(out, a.Scope)
		}
	}
	return out
}
func practicumEligible(p *entity.Principal, a entity.PracticumActivationRecord) bool {
	if practicumAuthenticated(p) != nil || !containsInt(a.Activation.StudentIDs, p.User.ID) {
		return false
	}
	for _, s := range practicumStudentScopes(p) {
		if s.Contains(a.Activation.Scope) {
			return true
		}
	}
	return false
}
func containsInt(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func practicumRead[T any](tx repository.PracticumTx, k repository.PracticumKind, id string, lock bool) (T, error) {
	var out T
	var row repository.PracticumRecord
	var err error
	if lock {
		row, err = tx.Lock(k, id)
	} else {
		row, err = tx.Get(k, id)
	}
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(row.Data, &out)
	return out, err
}
func practicumPut(tx repository.PracticumTx, k repository.PracticumKind, id, parent string, owner int64, scope entity.Scope, status, unique string, expires *time.Time, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return tx.Put(k, repository.PracticumRecord{ID: id, ParentID: parent, OwnerID: owner, Scope: scope, Status: status, UniqueKey: unique, ExpiresAt: expires, Data: raw})
}
func practicumAudit(tx repository.PracticumTx, p *entity.Principal, c PracticumCommand, action, id string, scope entity.Scope) error {
	if strings.HasPrefix(action, "practicum.attempt.") || strings.HasPrefix(action, "practicum.grade.") {
		row, err := tx.Get(repository.PracticumAttempts, id)
		if err == nil {
			parent, err := tx.Get(repository.PracticumActivations, row.ParentID)
			if err != nil {
				return err
			}
			scope = parent.Scope
		} else if !errors.Is(err, repository.ErrPracticumNotFound) {
			return err
		}
	}
	now, err := tx.Now()
	if err != nil {
		return err
	}
	var actor *int64
	if p != nil {
		actor = &p.User.ID
	}
	return tx.Audit(entity.AuditEvent{ActorID: actor, Action: action, App: "praktikum", SubjectType: "practicum", SubjectID: id, Scope: scope, RequestID: c.RequestID, OperationID: c.OperationID, CreatedAt: now, Detail: map[string]any{}})
}

// A replay resolves current domain state, rather than storing a stale response body.
func practicumMutate[T any](s *PracticumService, ctx context.Context, p *entity.Principal, c PracticumCommand, action, resource string, input any, fn func(repository.PracticumTx, string) (T, string, entity.Scope, error)) (T, error) {
	var out T
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !practicumKeyPattern.MatchString(c.Key) {
		return out, ErrValidation
	}
	opID := practicumHash([]any{p.User.ID, action, resource, c.Key})
	hash := practicumHash(input)
	var committed error
	err := s.options.Repository.Transaction(ctx, func(tx repository.PracticumTx) error {
		replay := ""
		op, err := practicumRead[practicumOperation](tx, repository.PracticumOperations, opID, false)
		if err == nil {
			if op.PayloadHash != hash {
				return repository.ErrConflict
			}
			replay = op.TargetID
		} else if !errors.Is(err, repository.ErrPracticumNotFound) {
			return err
		}
		var target string
		var scope entity.Scope
		out, target, scope, err = fn(tx, replay)
		if errors.Is(err, ErrPracticumClosed) {
			committed = err
			return nil
		}
		if err != nil {
			return err
		}
		if replay != "" {
			return nil
		}
		if err = practicumPut(tx, repository.PracticumOperations, opID, "", p.User.ID, scope, "", opID, nil, practicumOperation{PayloadHash: hash, TargetID: target}); err != nil {
			return err
		}
		return practicumAudit(tx, p, c, action, target, scope)
	})
	if err == nil {
		err = committed
	}
	return out, err
}
func validPracticumTitle(v string) bool {
	return len(strings.TrimSpace(v)) > 0 && len(v) <= 200 && !strings.ContainsRune(v, 0)
}
func validPracticumScope(s entity.Scope) bool {
	if s.Domain != "praktikum" || s.Shift < 0 || s.Shift > 2 {
		return false
	}
	for _, v := range []string{s.Institution, s.Division, s.ClassID, s.SessionID} {
		if len(v) > 128 || strings.ContainsAny(v, "\x00*\r\n") {
			return false
		}
	}
	return true
}
func validPracticumCourse(kind string, duration int) bool {
	return duration >= 1 && duration <= 180 && (kind == "pretest" || kind == "posttest" || kind == "keterampilan" || kind == "ujian_praktik")
}
func validPracticumQuestion(q PracticumQuestionInput) bool {
	if q.Type != "essay" && q.Type != "coding" || q.Difficulty != "easy" && q.Difficulty != "medium" && q.Difficulty != "hard" || q.Points < 1 || q.Points > 100 || len(q.Prompt) < 1 || len(q.Prompt) > 8192 || len(q.Category) > 128 || len(q.StarterCode) > 64<<10 || len(q.ReferenceAnswer) > 64<<10 || len(q.Language) > 32 || len(q.Rubric) < 1 || len(q.Rubric) > 20 {
		return false
	}
	raw, marshalErr := json.Marshal(q)
	if marshalErr != nil || len(raw) > 32<<10 {
		return false
	}
	if q.Type == "coding" && q.Language == "" {
		return false
	}
	sum := 0
	seen := map[string]bool{}
	for _, r := range q.Rubric {
		if r.ID == "" || len(r.ID) > 128 || seen[r.ID] || r.MaxScore < 0 || r.MaxScore > 100 || len(r.Description) < 1 || len(r.Description) > 2000 {
			return false
		}
		seen[r.ID] = true
		sum += r.MaxScore
	}
	return sum == q.Points
}
func practicumPublicQuestion(q entity.PracticumQuestion) entity.PracticumQuestion {
	q.ReferenceAnswer = ""
	q.Rubric = nil
	q.CourseID = ""
	q.Version = 0
	return q
}
func practicumPublicActivation(a entity.PracticumActivation, admin bool) entity.PracticumActivation {
	a.AccessToken = ""
	if !admin {
		a.StudentIDs = nil
		a.EntryApprovedIDs = nil
		a.QuestionIDs = nil
	}
	return a
}
func practicumAttemptView(a entity.PracticumAttemptRecord, now time.Time) entity.PracticumAttempt {
	out := a.Attempt
	out.ServerNow = now.UTC()
	out.Timezone = "Asia/Jakarta"
	out.TimezoneOffset = "+07:00"
	out.Questions = append([]entity.PracticumQuestion{}, a.Attempt.Questions...)
	out.Answers = append([]entity.PracticumAnswer{}, a.Attempt.Answers...)
	for i := range out.Questions {
		out.Questions[i] = practicumPublicQuestion(out.Questions[i])
	}
	if out.Status != "graded" {
		out.Score = nil
		out.Percentage = nil
		for i := range out.Answers {
			out.Answers[i].Score = nil
			out.Answers[i].Feedback = ""
		}
	}
	return out
}
func practicumAttemptKey(activation string, user int64) string {
	return fmt.Sprintf("%s:%d", activation, user)
}
func practicumEqualToken(hash, token string) bool {
	candidate := security.Hash(token)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(candidate)) == 1
}
