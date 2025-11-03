// Helper функции для быстрого использования
package sqliteerrors

// IsConstraint проверяет constraint violation
func IsConstraint(err error) bool {
	return E(err).Constraint().Is()
}

// IsForeignKey проверяет FK violation
func IsForeignKey(err error) bool {
	return E(err).ForeignKey().Is()
}

// IsUnique проверяет unique constraint violation
func IsUnique(err error) bool {
	return E(err).Unique().Is()
}

// IsPrimaryKey проверяет PK violation
func IsPrimaryKey(err error) bool {
	return E(err).PrimaryKey().Is()
}

// IsSyntax проверяет синтаксическую ошибку
func IsSyntax(err error) bool {
	return E(err).Syntax().Is()
}

// IsBusy проверяет busy error
func IsBusy(err error) bool {
	return E(err).Busy().Is()
}

// IsDataIntegrityViolation проверяет все виды violations целостности данных
func IsDataIntegrityViolation(err error) bool {
	return E(err).Constraint().Is()
}

// IsCriticalError проверяет критические ошибки (не violations)
func IsCriticalError(err error) bool {
	analyzer := E(err)
	return analyzer.Syntax().Is() ||
		analyzer.Busy().Is() ||
		analyzer.Locked().Is() ||
		analyzer.IO().Is() ||
		analyzer.Corrupt().Is() ||
		analyzer.Full().Is() ||
		analyzer.CantOpen().Is()
}

// ShouldAbort определяет нужно ли прерывать выполнение
func ShouldAbort(err error) bool {
	return IsCriticalError(err) || E(err).Schema().Is()
}

// GetErrorType возвращает тип ошибки для классификации
func GetErrorType(err error) string {
	analyzer := E(err)

	switch {
	case analyzer.ForeignKey().Is():
		return "FOREIGN_KEY_VIOLATION"
	case analyzer.Unique().Is():
		return "UNIQUE_CONSTRAINT_VIOLATION"
	case analyzer.PrimaryKey().Is():
		return "PRIMARY_KEY_VIOLATION"
	case analyzer.Syntax().Is():
		return "SYNTAX_ERROR"
	case analyzer.Busy().Is():
		return "BUSY_ERROR"
	case analyzer.Locked().Is():
		return "LOCKED_ERROR"
	case analyzer.IO().Is():
		return "IO_ERROR"
	case analyzer.Corrupt().Is():
		return "CORRUPT_ERROR"
	case analyzer.Constraint().Is():
		return "CONSTRAINT_VIOLATION"
	default:
		return "OTHER_ERROR"
	}
}
