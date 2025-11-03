package sqliteerrors

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/mattn/go-sqlite3"
)

// Error implement sqlite error code.

func TestSQLErrorAnalyzer(t *testing.T) {

	// * это грязный хак использования unsafe.Pointer
	e := sqlite3.Error{Code: sqlite3.ErrConstraint}
	field := reflect.ValueOf(&e).Elem().FieldByName("err")
	pointer := unsafe.Pointer(field.UnsafeAddr())
	msgPtr := (*string)(pointer)
	*msgPtr = "FOREIGN KEY constraint failed"
	fmt.Printf("%+v", e)

	tests := []struct {
		name     string
		err      error
		check    func(*SQLErrorAnalyzer) *SQLErrorAnalyzer
		expected bool
	}{
		{
			name: "foreign key violation",
			err: func() error {
				e := sqlite3.Error{Code: sqlite3.ErrConstraint}
				field := reflect.ValueOf(&e).Elem().FieldByName("err")
				pointer := unsafe.Pointer(field.UnsafeAddr())
				msgPtr := (*string)(pointer)
				*msgPtr = "FOREIGN KEY constraint failed"
				return e
			}(),
			check: func(a *SQLErrorAnalyzer) *SQLErrorAnalyzer {
				return a.ForeignKey()
			},
			expected: true,
		},
		{
			name: "unique constraint violation",
			err: func() error {
				e := sqlite3.Error{Code: sqlite3.ErrConstraint}
				field := reflect.ValueOf(&e).Elem().FieldByName("err")
				pointer := unsafe.Pointer(field.UnsafeAddr())
				msgPtr := (*string)(pointer)
				*msgPtr = "UNIQUE constraint failed"
				return e
			}(),
			check: func(a *SQLErrorAnalyzer) *SQLErrorAnalyzer {
				return a.Unique()
			},
			expected: true,
		},
		{
			name: "syntax error by text",
			err: func() error {
				e := sqlite3.Error{Code: sqlite3.ErrError}
				field := reflect.ValueOf(&e).Elem().FieldByName("err")
				pointer := unsafe.Pointer(field.UnsafeAddr())
				msgPtr := (*string)(pointer)
				*msgPtr = "4 values for 5 columns"
				return e
			}(),
			check: func(a *SQLErrorAnalyzer) *SQLErrorAnalyzer {
				return a.Syntax()
			},
			expected: true,
		},
		{
			name: "busy error",
			err:  sqlite3.Error{Code: sqlite3.ErrBusy},
			check: func(a *SQLErrorAnalyzer) *SQLErrorAnalyzer {
				return a.Busy()
			},
			expected: true,
		},
		//TODO: go-sqlite3.Error {Code: 1, ExtendedCode: 1, SystemErrno: 0, err: "no such table: virtual_services"}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := E(tt.err)
			result := tt.check(analyzer).Is()

			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	fkErr := sqlite3.Error{Code: sqlite3.ErrConstraint}
	field := reflect.ValueOf(&fkErr).Elem().FieldByName("err")
	pointer := unsafe.Pointer(field.UnsafeAddr())
	msgPtr := (*string)(pointer)
	*msgPtr = "FOREIGN KEY constraint failed"

	syntaxErr := sqlite3.Error{Code: sqlite3.ErrError}
	field1 := reflect.ValueOf(&syntaxErr).Elem().FieldByName("err")
	pointer1 := unsafe.Pointer(field1.UnsafeAddr())
	msgPtr1 := (*string)(pointer1)
	*msgPtr1 = "syntax error near 'FROM'"

	if !IsForeignKey(fkErr) {
		t.Error("IsForeignKey should return true for FK violation")
	}

	if !IsSyntax(syntaxErr) {
		t.Error("IsSyntax should return true for syntax errors")
	}

	if !ShouldAbort(syntaxErr) {
		t.Error("ShouldAbort should return true for syntax errors")
	}

	if ShouldAbort(fkErr) {
		t.Error("ShouldAbort should return false for FK violations")
	}
}
