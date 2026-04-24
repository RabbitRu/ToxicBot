package chathistory

import "time"

type Entry struct {
	ID        int
	Time      time.Time
	Author    string
	Text      string
	ReplyToID int
	FromBot   bool
}
