package activity

import "time"

// Event 是活动记录事件，描述客户端在 TeamSpeak 服务器上的状态跃迁。
type Event struct {
	ID              int64     `json:"id"`
	Time            time.Time `json:"time"`
	Action          string    `json:"action"` // join, leave, move, rename
	UID             string    `json:"uid"`
	Nickname        string    `json:"nickname"`
	TargetNickname  string    `json:"target_nickname"`
	ChannelID       int       `json:"channel_id"`
	ChannelName     string    `json:"channel_name"`
	FromChannelID   int       `json:"from_channel_id"`
	FromChannelName string    `json:"from_channel_name"`
}

// PageResult 是分页查询返回的包装对象。
type PageResult struct {
	Items      []Event `json:"items"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
	Total      int     `json:"total"`
	TotalPages int     `json:"total_pages"`
}
