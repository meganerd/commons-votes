// internal/model/vote.go
package model

type Vote struct {
	ID       int
	BillID   int
	MemberID int
	Result   string // Yea / Nay / Abstain
	VoteDate string // ISO date (YYYY-MM-DD)
}
