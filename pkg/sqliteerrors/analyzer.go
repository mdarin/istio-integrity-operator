// Fluent Error Analyzer
package sqliteerrors

import (
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// SQLErrorAnalyzer - fluent интерфейс для анализа ошибок SQLite
type SQLErrorAnalyzer struct {
	err       error
	sqliteErr *sqlite3.Error
	matched   bool
}

// E создает новый анализатор ошибок
func E(err error) *SQLErrorAnalyzer {
	analyzer := &SQLErrorAnalyzer{err: err}
	if sqliteErr, ok := err.(sqlite3.Error); ok {
		analyzer.sqliteErr = &sqliteErr
	}
	return analyzer
}

// Constraint проверяет что ошибка связана с constraints
func (a *SQLErrorAnalyzer) Constraint() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrConstraint {
		a.matched = true
	}
	return a
}

// ForeignKey проверяет FK violation
func (a *SQLErrorAnalyzer) ForeignKey() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrConstraint {
		// Дополнительная проверка по тексту для точности
		if strings.Contains(a.err.Error(), "FOREIGN KEY") {
			a.matched = true
		}
	}
	return a
}

// Unique проверяет unique constraint violation
func (a *SQLErrorAnalyzer) Unique() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrConstraint {
		if strings.Contains(a.err.Error(), "UNIQUE") {
			a.matched = true
		}
	}
	return a
}

// PrimaryKey проверяет primary key violation
func (a *SQLErrorAnalyzer) PrimaryKey() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrConstraint {
		if strings.Contains(a.err.Error(), "PRIMARY KEY") {
			a.matched = true
		}
	}
	return a
}

// Violation общая проверка constraint violations
func (a *SQLErrorAnalyzer) Violation() *SQLErrorAnalyzer {
	return a.Constraint()
}

// Syntax проверяет синтаксические ошибки
func (a *SQLErrorAnalyzer) Syntax() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	// SQLite не имеет отдельного кода для синтаксических ошибок,
	// но мы можем анализировать по тексту
	errText := a.err.Error()
	if strings.Contains(errText, "syntax error") ||
		strings.Contains(errText, "values for") && strings.Contains(errText, "columns") ||
		strings.Contains(errText, "no such table") ||
		strings.Contains(errText, "no such column") {
		a.matched = true
	}
	return a
}

// Busy проверяет что база занята
func (a *SQLErrorAnalyzer) Busy() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrBusy {
		a.matched = true
	}
	return a
}

// Locked проверяет блокировки
func (a *SQLErrorAnalyzer) Locked() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrLocked {
		a.matched = true
	}
	return a
}

// IO проверяет ошибки ввода-вывода
func (a *SQLErrorAnalyzer) IO() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrIoErr {
		a.matched = true
	}
	return a
}

// Corrupt проверяет corruption базы данных
func (a *SQLErrorAnalyzer) Corrupt() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrCorrupt {
		a.matched = true
	}
	return a
}

// Full проверяет переполнение базы
func (a *SQLErrorAnalyzer) Full() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrFull {
		a.matched = true
	}
	return a
}

// CantOpen проверяет ошибки открытия базы
func (a *SQLErrorAnalyzer) CantOpen() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrCantOpen {
		a.matched = true
	}
	return a
}

// Schema проверяет изменения схемы
func (a *SQLErrorAnalyzer) Schema() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrSchema {
		a.matched = true
	}
	return a
}

// Mismatch проверяет type mismatch
func (a *SQLErrorAnalyzer) Mismatch() *SQLErrorAnalyzer {
	if a.matched || a.sqliteErr == nil {
		return a
	}

	if a.sqliteErr.Code == sqlite3.ErrMismatch {
		a.matched = true
	}
	return a
}

// Is возвращает true если ошибка соответствует проверкам
func (a *SQLErrorAnalyzer) Is() bool {
	return a.matched
}

// Not инвертирует проверку
func (a *SQLErrorAnalyzer) Not() bool {
	return !a.matched
}

// Error возвращает оригинальную ошибку
func (a *SQLErrorAnalyzer) Error() error {
	return a.err
}

// Code возвращает SQLite error code
func (a *SQLErrorAnalyzer) Code() int {
	if a.sqliteErr != nil {
		return int(a.sqliteErr.Code)
	}
	return 0
}

// String возвращает текстовое представление
func (a *SQLErrorAnalyzer) String() string {
	if a.sqliteErr != nil {
		return fmt.Sprintf("SQLiteError[%d]: %s", a.sqliteErr.Code, a.sqliteErr.Error())
	}
	if a.err != nil {
		return fmt.Sprintf("Error: %s", a.err.Error())
	}
	return "No error"
}
