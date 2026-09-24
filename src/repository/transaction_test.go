package repository

import (
	"errors"
	"testing"
)

type postgresStateError struct{ state string }

func (e postgresStateError) Error() string { return e.state }
func (e postgresStateError) Field(field byte) string {
	if field == 'C' {
		return e.state
	}
	return ""
}

func TestRetryableTransactionErrors(t *testing.T) {
	for _, state := range []string{"40001", "40P01"} {
		err := errors.Join(errors.New("transaction failed"), postgresStateError{state: state})
		if !isRetryableTransactionError(err) {
			t.Fatalf("SQLSTATE %s was not retryable", state)
		}
	}
	for _, err := range []error{
		postgresStateError{state: "23505"},
		errors.New("ordinary failure"),
	} {
		if isRetryableTransactionError(err) {
			t.Fatalf("error %v was unexpectedly retryable", err)
		}
	}
}
