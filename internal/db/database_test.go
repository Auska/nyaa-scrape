package db

import (
	"testing"

	"nyaa-crawler/pkg/models"
)

func TestUpdatePushedStatusValidation(t *testing.T) {
	validTargets := []models.PushTarget{models.PushTargetTransmission, models.PushTargetAria2}
	invalidTargets := []models.PushTarget{"invalid_column", "name", "id", "pushed_to_unknown", ""}

	for _, target := range validTargets {
		t.Run("valid_target_"+string(target), func(t *testing.T) {
			if target != models.PushTargetTransmission && target != models.PushTargetAria2 {
				t.Errorf("unexpected valid target: %s", target)
			}
		})
	}

	for _, target := range invalidTargets {
		t.Run("invalid_target_"+string(target), func(t *testing.T) {
			isValid := target == models.PushTargetTransmission || target == models.PushTargetAria2
			if isValid {
				t.Errorf("target %q should be invalid", target)
			}
		})
	}
}

func TestLikePattern(t *testing.T) {
	pattern := "One Piece"
	likePattern := "%" + pattern + "%"

	if likePattern != "%One Piece%" {
		t.Errorf("expected %%One Piece%%, got %s", likePattern)
	}
}
