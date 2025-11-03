package integrity

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type IntegrityTestSuite struct {
	suite.Suite
	operator *SQLiteIntegrityOperator
}

func TestIntegritySuite(t *testing.T) {
	suite.Run(t, new(IntegrityTestSuite))
}

func (s *IntegrityTestSuite) SetupTest() {
	s.operator = &SQLiteIntegrityOperator{}
}
