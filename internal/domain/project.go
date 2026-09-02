package domain

import "time"

type Project struct {
	ID            string
	Name          string
	IntakeProfile IntakeProfile
	CreatedAt     time.Time
}
