package usecase

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/algohub/algohub-rewrite/backend/internal/authorization"
	"github.com/algohub/algohub-rewrite/backend/internal/entity"
	"github.com/algohub/algohub-rewrite/backend/internal/repository"
)

func (s *AcademicService) FinanceMutations(ctx context.Context, p *entity.Principal, admin bool, page PracticumPage) ([]entity.AcademicFinanceMutation, error) {
	out := []entity.AcademicFinanceMutation{}
	if err := practicumAuthenticated(p); err != nil {
		return nil, err
	}
	if !page.valid() {
		return nil, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		var filter repository.AcademicFilter
		if admin {
			scopes := academicFilterScopes(p, academicFinanceManage)
			if len(scopes) == 0 {
				return nil
			}
			filter = repository.AcademicFilter{Scopes: scopes, Limit: page.Limit, Offset: page.Offset}
		} else {
			filter = repository.AcademicFilter{OwnerID: p.User.ID, Limit: page.Limit, Offset: page.Offset}
		}
		rows, err := tx.List(repository.AcademicFinanceMutations, filter)
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rec entity.AcademicFinanceMutation
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			out = append(out, rec)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
		return nil
	})
	return out, err
}

// FinanceBalance returns the caller's own saldo plus a bounded mutation list.
// Student saldo is the signed sum of posted mutations owned by that user.
func (s *AcademicService) FinanceBalance(ctx context.Context, p *entity.Principal, page PracticumPage) (entity.AcademicFinanceBalance, error) {
	out := entity.AcademicFinanceBalance{UserID: p.User.ID, UserName: p.User.Name, Mutations: []entity.AcademicFinanceMutation{}, Timezone: "Asia/Jakarta"}
	if err := practicumAuthenticated(p); err != nil {
		return out, err
	}
	if !page.valid() {
		return out, ErrValidation
	}
	err := s.options.Repository.Transaction(ctx, func(tx repository.AcademicTx) error {
		rows, err := tx.List(repository.AcademicFinanceMutations, repository.AcademicFilter{OwnerID: p.User.ID, Limit: page.Limit, Offset: page.Offset})
		if err != nil {
			return err
		}
		for _, r := range rows {
			var rec entity.AcademicFinanceMutation
			if err = json.Unmarshal(r.Data, &rec); err != nil {
				return err
			}
			out.Mutations = append(out.Mutations, rec)
			switch rec.Type {
			case "income":
				out.Income += rec.Amount
			case "expense":
				out.Expense += rec.Amount
			case "adjustment":
				out.Adjust += rec.Amount
			}
		}
		sort.Slice(out.Mutations, func(i, j int) bool { return out.Mutations[i].CreatedAt.After(out.Mutations[j].CreatedAt) })
		out.Saldo = out.Income - out.Expense + out.Adjust
		return nil
	})
	return out, err
}

// PostFinanceMutation is the admin-side ledger write: set a target user's
// balance (adjustment to a fixed value) or record income/expense against them.
func (s *AcademicService) PostFinanceMutation(ctx context.Context, p *entity.Principal, c AcademicCommand, in AcademicFinanceMutationInput) (entity.AcademicFinanceMutation, error) {
	if in.TargetUserID < 1 || !validFinanceType(in.Type) || len(in.Note) < 1 || len(in.Note) > 500 || in.Amount < 0 || in.Amount > 100000000000 || len(in.Category) > 64 {
		return entity.AcademicFinanceMutation{}, ErrValidation
	}
	if in.Type == "adjustment" && in.Amount == 0 {
		return entity.AcademicFinanceMutation{}, ErrValidation
	}
	return academicMutate(s, ctx, p, c, "finance.mutation.create", in.Note, in, func(tx repository.AcademicTx, replay string) (out entity.AcademicFinanceMutation, target string, scope entity.Scope, err error) {
		scopes := academicFilterScopes(p, academicFinanceManage)
		if len(scopes) == 0 {
			err = authorization.ErrForbidden
			return
		}
		scope = scopes[0]
		if replay != "" {
			out, err = academicReadRecord[entity.AcademicFinanceMutation](tx, repository.AcademicFinanceMutations, replay, false)
			return out, replay, out.Scope, err
		}
		profile, err := s.options.IdentityRepository.FindUserByID(ctx, in.TargetUserID)
		if err != nil {
			return entity.AcademicFinanceMutation{}, "", scope, err
		}
		now, err := tx.Now()
		if err != nil {
			return entity.AcademicFinanceMutation{}, "", scope, err
		}
		id := academicID("fin")
		out = entity.AcademicFinanceMutation{
			ID:        id,
			UserID:    profile.ID,
			UserName:  profile.Name,
			Type:      in.Type,
			Note:      strings.TrimSpace(in.Note),
			Amount:    in.Amount,
			Category:  strings.TrimSpace(in.Category),
			RentalID:  in.RentalID,
			CreatedAt: now,
			Scope:     scope,
			Version:   1,
		}
		target = id
		err = academicPut(tx, repository.AcademicFinanceMutations, id, "", profile.ID, scope, "posted", "", nil, out)
		return
	})
}

func validFinanceType(t string) bool {
	switch t {
	case "income", "expense", "adjustment":
		return true
	}
	return false
}
