package database

import "time"

// Theme 主题数据
type Theme struct {
	Id         int       `xorm:"id autoincr pk"`
	Keyword    string    `xorm:"keyword"`
	Value      string    `xorm:"value longtext"`
	CreateTime time.Time `xorm:"'create_time' created"`
	UpdateTime time.Time `xorm:"'update_time' updated"`
}

func (Theme) TableName() string {
	return "theme"
}
