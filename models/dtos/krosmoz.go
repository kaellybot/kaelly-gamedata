package dtos

import "fmt"

// AlmanaxMismatch is a crawled day that could not be joined to the client data.
type AlmanaxMismatch struct {
	Month  int
	Day    int
	Reason string
}

func (mismatch AlmanaxMismatch) String() string {
	return fmt.Sprintf("%02d-%02d: %s", mismatch.Month, mismatch.Day, mismatch.Reason)
}

// KrosmozAlmanax is the Dofus Touch block of one Krosmoz almanax page.
type KrosmozAlmanax struct {
	Month            int
	Day              int
	BonusName        string
	BonusDescription string
	OfferingName     string
	OfferingQuantity int32
	OfferingIconID   int32
}
