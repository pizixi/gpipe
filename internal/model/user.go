package model

import "time"

type User struct {
	ID             uint32     `json:"id"`
	Remark         string     `json:"remark"`
	Key            string     `json:"key"`
	CreateTime     time.Time  `json:"create_time"`
	LastOnlineTime *time.Time `json:"last_online_time,omitempty"`
	LastIP         string     `json:"last_ip"`
}

type PlayerUpdate struct {
	ID     uint32
	Remark string
	Key    string
}
