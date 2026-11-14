package main

import (
	"encoding/json"
	"errors"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

// normalizeForTable re-encodes a bound payload through the entity type the
// application uses. This makes the stored jsonb identical to what a read/write
// round-trip produces, so immutable-freeze triggers do not see a phantom change
// caused by omitted empty optional fields.
func normalizeForTable(table string, scope, data []byte) ([]byte, []byte, error) {
	var s entity.Scope
	if err := json.Unmarshal(scope, &s); err != nil {
		return nil, nil, err
	}
	normalizedScope, err := json.Marshal(s)
	if err != nil {
		return nil, nil, err
	}
	var target any
	switch table {
	case "learning_levels":
		target = &entity.LearningLevel{}
	case "learning_modules":
		target = &entity.LearningModule{}
	case "learning_lessons":
		target = &entity.LearningLesson{}
	case "learning_progress":
		target = &entity.LearningProgress{}
	case "learning_profiles":
		target = &entity.LearningProfile{}
	case "practicum_sessions":
		target = &entity.PracticumSession{}
	case "practicum_courses":
		target = &entity.PracticumCourse{}
	case "practicum_questions":
		target = &entity.PracticumQuestion{}
	case "practicum_activations":
		target = &entity.PracticumActivationRecord{}
	case "practicum_attempts":
		target = &entity.PracticumAttemptRecord{}
	case "practicum_grading_jobs":
		target = &entity.PracticumGradingJob{}
	case "academic_classes":
		target = &entity.AcademicClass{}
	case "academic_schedules":
		target = &entity.AcademicSchedule{}
	case "academic_inventory_items":
		target = &entity.InventoryItem{}
	default:
		return nil, nil, errors.New("no entity contract for table")
	}
	if err = json.Unmarshal(data, target); err != nil {
		return nil, nil, err
	}
	normalizedData, err := json.Marshal(target)
	if err != nil {
		return nil, nil, err
	}
	return normalizedScope, normalizedData, nil
}
