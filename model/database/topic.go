package database

import "time"

// Topic 主题数据
type Topic struct {
	Id         int       `xorm:"id autoincr pk"`
	Keyword    string    `xorm:"keyword"`
	Value      string    `xorm:"value longtext"`
	Image      string    `xorm:"image"` // 新增字段，用于存储图片路径
	CreateTime time.Time `xorm:"'create_time' created"`
	UpdateTime time.Time `xorm:"'update_time' updated"`
}

func (Topic) TableName() string {
	return "topic"
}
