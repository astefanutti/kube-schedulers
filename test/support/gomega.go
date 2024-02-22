package support

import (
	"time"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var (
	TimeoutShort  = 1 * time.Minute
	TimeoutMedium = 2 * time.Minute
	TimeoutLong   = 5 * time.Minute
)

func init() {
	gomega.SetDefaultEventuallyTimeout(TimeoutShort)
	gomega.SetDefaultEventuallyPollingInterval(1 * time.Second)
	gomega.SetDefaultConsistentlyDuration(30 * time.Second)
	gomega.SetDefaultConsistentlyPollingInterval(1 * time.Second)
	// Disable object truncation on test results
	format.MaxLength = 0
}
