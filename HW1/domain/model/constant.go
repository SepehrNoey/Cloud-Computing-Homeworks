package model

type Status string

const (
	Pending Status = "pending"
	Failure Status = "failure"
	Ready   Status = "ready"
	Done    Status = "done"
)

const Unknown = "unknown"
