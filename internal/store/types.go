package store

import "time"

// Snapshot 是 ServerQuery 轮询结果的完整快照，供 store 持有、api 序列化、tsquery 产出。
type Snapshot struct {
	Server        ServerInfo        `json:"server"`
	Channels      []ChannelInfo     `json:"channels"`
	Clients       []ClientInfo      `json:"clients"`
	ServerGroups  []ServerGroupInfo  `json:"server_groups"`
	ChannelGroups []ChannelGroupInfo `json:"channel_groups"`
	Meta          SnapshotMeta      `json:"meta"`
}

type SnapshotMeta struct {
	Sequence  uint64    `json:"sequence"`
	FetchedAt time.Time `json:"fetched_at"`
	LatencyMS int64     `json:"latency_ms"`
	Stale     bool      `json:"stale"`
	LastError string    `json:"last_error,omitempty"`
}

type ServerInfo struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	Platform         string `json:"platform"`
	Version          string `json:"version"`
	UniqueIdentifier string `json:"unique_identifier"`
	ChannelsOnline   int    `json:"channels_online"`
	ClientsOnline    int    `json:"clients_online"`
	DisplayedClients int    `json:"displayed_clients"`
	MaxClients       int    `json:"max_clients"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
	CreatedAt        int64  `json:"created_at"`
}

type ChannelInfo struct {
	ID            int    `json:"id"`
	ParentID      int    `json:"parent_id"`
	Order         int    `json:"order"`
	Name          string `json:"name"`
	Topic         string `json:"topic,omitempty"`
	TotalClients  int    `json:"total_clients"`
	MaxClients    int    `json:"max_clients"`
	SecondsEmpty  int    `json:"seconds_empty"`
	Permanent     bool   `json:"permanent"`
	SemiPermanent bool   `json:"semi_permanent"`
	Default       bool   `json:"default"`
	Password      bool   `json:"password"`
}

type ClientInfo struct {
	ID               int    `json:"id"`
	DatabaseID       int    `json:"database_id"`
	ChannelID        int    `json:"channel_id"`
	Nickname         string `json:"nickname"`
	UniqueID         string `json:"unique_id,omitempty"`
	Type             int    `json:"type"`
	Country          string `json:"country,omitempty"`
	Away             bool   `json:"away"`
	AwayMessage      string `json:"away_message,omitempty"`
	InputMuted       bool   `json:"input_muted"`
	OutputMuted      bool   `json:"output_muted"`
	OutputOnlyMuted  bool   `json:"output_only_muted"`
	InputHardware    bool   `json:"input_hardware"`
	OutputHardware   bool   `json:"output_hardware"`
	Talking          bool   `json:"talking"`
	TalkPower        int    `json:"talk_power"`
	GroupSortID      int    `json:"group_sort_id"`
	IdleSeconds      int64  `json:"idle_seconds"`
	ConnectedSeconds int64  `json:"connected_seconds"`
	ServerGroups     string `json:"server_groups,omitempty"`
	ChannelGroupID   int    `json:"channel_group_id"`
}

type ServerGroupInfo struct {
	SGID    int    `json:"sgid"`
	Name    string `json:"name"`
	SortID  int    `json:"sort_id"`
	IconID  uint32 `json:"icon_id"`
	IconURL string `json:"icon_url,omitempty"`
}

type ChannelGroupInfo struct {
	CGID    int    `json:"cgid"`
	Name    string `json:"name"`
	SortID  int    `json:"sort_id"`
	IconID  uint32 `json:"icon_id"`
	IconURL string `json:"icon_url,omitempty"`
}
