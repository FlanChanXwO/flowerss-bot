package model

type Subscribe struct {
	ID                 uint `gorm:"primary_key;AUTO_INCREMENT"`
	UserID             int64
	ThreadID           int  `gorm:"not null;default:0"`
	Paused             bool `gorm:"not null;default:false"`
	SourceID           uint
	EnableNotification int
	EnableTelegraph    int
	Tag                string
	Interval           int
	WaitTime           int
	EditTime
}
