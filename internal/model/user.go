package model

import "time"

type User struct {
	ID             uint32     `json:"id"`
	Remark         string     `json:"remark"`
	Key            string     `json:"key"`
	CreateTime     time.Time  `json:"create_time"`
	LastOnlineTime *time.Time `json:"last_online_time,omitempty"`
	ClientVersion  string     `json:"client_version"`
	ClientPlatform string     `json:"client_platform"`
	UpdaterVersion uint32     `json:"updater_version"`
}

type PlayerUpdate struct {
	ID     uint32
	Remark string
	Key    string
}
