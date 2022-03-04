package models

type Subscribe struct {
	Type         string       `json:"type,omitempty"`
	Subscription Subscription `json:"subscription,omitempty"`
}

type Subscription struct {
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Limit int    `json:"limit,omitempty"`
}
