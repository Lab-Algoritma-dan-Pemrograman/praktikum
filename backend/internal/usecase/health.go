package usecase

import "context"

type ReadinessService struct {
	repository readinessRepository
}

type readinessRepository interface {
	Check(context.Context) error
}

func NewReadinessService(repository readinessRepository) ReadinessService {
	return ReadinessService{repository: repository}
}

func (s ReadinessService) Check(ctx context.Context) error {
	return s.repository.Check(ctx)
}
