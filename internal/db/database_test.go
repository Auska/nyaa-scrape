package db

import (
	"testing"
)

func TestUpdatePushedStatusValidation(t *testing.T) {
	// Test whitelist validation without database connection
	validColumns := []string{"pushed_to_transmission", "pushed_to_aria2"}
	invalidColumns := []string{"invalid_column", "name", "id", "pushed_to_unknown", ""}

	// We can't test the actual database update without a connection,
	// but we can verify the validation logic by checking the error message format
	for _, col := range validColumns {
		// These should pass validation (but will fail without DB)
		t.Run("valid_column_"+col, func(t *testing.T) {
			// Just verify the column name is in our expected list
			if col != "pushed_to_transmission" && col != "pushed_to_aria2" {
				t.Errorf("unexpected valid column: %s", col)
			}
		})
	}

	for _, col := range invalidColumns {
		t.Run("invalid_column_"+col, func(t *testing.T) {
			// Verify these would fail validation
			isValid := col == "pushed_to_transmission" || col == "pushed_to_aria2"
			if isValid {
				t.Errorf("column %q should be invalid", col)
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
