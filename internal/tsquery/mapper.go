package tsquery

import (
	"math"
	"sort"
	"strings"

	"tspeek/internal/store"
)

func mapServer(row map[string]string) store.ServerInfo {
	return store.ServerInfo{
		Name:             row["virtualserver_name"],
		Status:           row["virtualserver_status"],
		Platform:         row["virtualserver_platform"],
		Version:          row["virtualserver_version"],
		UniqueIdentifier: row["virtualserver_unique_identifier"],
		ChannelsOnline:   parseInt(row["virtualserver_channelsonline"]),
		ClientsOnline:    parseInt(row["virtualserver_clientsonline"]),
		MaxClients:       parseInt(row["virtualserver_maxclients"]),
		UptimeSeconds:    parseInt64(row["virtualserver_uptime"]),
		CreatedAt:        parseInt64(row["virtualserver_created"]),
	}
}

func mapChannels(rows []map[string]string) []store.ChannelInfo {
	channels := make([]store.ChannelInfo, 0, len(rows))
	for _, row := range rows {
		channels = append(channels, store.ChannelInfo{
			ID:            parseInt(row["cid"]),
			ParentID:      parseInt(row["pid"]),
			Order:         parseInt(row["channel_order"]),
			Name:          row["channel_name"],
			Topic:         row["channel_topic"],
			TotalClients:  parseInt(row["total_clients"]),
			MaxClients:    parseInt(row["channel_maxclients"]),
			SecondsEmpty:  parseInt(row["seconds_empty"]),
			Permanent:     parseBool(row["channel_flag_permanent"]),
			SemiPermanent: parseBool(row["channel_flag_semi_permanent"]),
			Default:       parseBool(row["channel_flag_default"]),
			Password:      parseBool(row["channel_flag_password"]),
		})
	}

	return sortChannelsByLinkedList(channels)
}

// sortChannelsByLinkedList 按 TeamSpeak 链表顺序排列频道。
// channel_order 存储的是"排在我前面的频道 ID"（前驱指针），0 表示排在最前。
func sortChannelsByLinkedList(channels []store.ChannelInfo) []store.ChannelInfo {
	// 按 ParentID 分组
	groups := make(map[int][]store.ChannelInfo)
	for _, ch := range channels {
		groups[ch.ParentID] = append(groups[ch.ParentID], ch)
	}

	// 收集并排序 parentID 以保证输出确定性
	parentIDs := make([]int, 0, len(groups))
	for pid := range groups {
		parentIDs = append(parentIDs, pid)
	}
	sort.Ints(parentIDs)

	result := make([]store.ChannelInfo, 0, len(channels))
	for _, pid := range parentIDs {
		group := groups[pid]
		// 建立索引：order（前驱 ID）→ channel
		byOrder := make(map[int]store.ChannelInfo, len(group))
		for _, ch := range group {
			byOrder[ch.Order] = ch
		}

		// 从 order=0 的头节点开始遍历链表
		sorted := make([]store.ChannelInfo, 0, len(group))
		visited := make(map[int]bool, len(group))
		if head, ok := byOrder[0]; ok {
			sorted = append(sorted, head)
			visited[head.ID] = true
			current := head
			for {
				next, ok := byOrder[current.ID]
				if !ok || visited[next.ID] {
					break
				}
				sorted = append(sorted, next)
				visited[next.ID] = true
				current = next
			}
		}

		// 容错：将未被链表遍历到的频道追加到末尾
		for _, ch := range group {
			if !visited[ch.ID] {
				sorted = append(sorted, ch)
			}
		}

		result = append(result, sorted...)
	}

	return result
}

// groupPerms 存储从 servergrouppermlist 获取的服务器组权限。
type groupPerms struct {
	sortID    int
	talkPower int
}

// mapGroupPerms 从 servergrouppermlist 响应中提取排序相关权限。
func mapGroupPerms(rows []map[string]string) groupPerms {
	var p groupPerms
	for _, row := range rows {
		switch row["permsid"] {
		case "i_group_sort_id":
			p.sortID = parseInt(row["permvalue"])
		case "i_client_talk_power":
			p.talkPower = parseInt(row["permvalue"])
		}
	}
	return p
}

func mapClients(rows []map[string]string, allGroupPerms map[int]groupPerms) []store.ClientInfo {
	clients := make([]store.ClientInfo, 0, len(rows))
	for _, row := range rows {
		clientType := parseInt(row["client_type"])
		// 过滤 ServerQuery 客户端（type=1）
		if clientType == 1 {
			continue
		}

		talkPower, sortID := resolveGroupPerms(row["client_servergroups"], allGroupPerms)

		clients = append(clients, store.ClientInfo{
			ID:               parseInt(row["clid"]),
			DatabaseID:       parseInt(row["client_database_id"]),
			ChannelID:        parseInt(row["cid"]),
			Nickname:         row["client_nickname"],
			UniqueID:         row["client_unique_identifier"],
			Type:             clientType,
			Country:          row["client_country"],
			Away:             parseBool(row["client_away"]),
			AwayMessage:      row["client_away_message"],
			InputMuted:       parseBool(row["client_input_muted"]),
			OutputMuted:      parseBool(row["client_output_muted"]),
			OutputOnlyMuted:  parseBool(row["client_outputonly_muted"]),
			InputHardware:    parseBool(row["client_input_hardware"]),
			OutputHardware:   parseBool(row["client_output_hardware"]),
			Talking:          parseBool(row["client_flag_talking"]),
			TalkPower:        talkPower,
			GroupSortID:      sortID,
			IdleSeconds:      parseInt64(row["client_idle_time"]) / 1000,
			ConnectedSeconds: parseInt64(row["client_connected_time"]) / 1000,
			ServerGroups:     row["client_servergroups"],
			ChannelGroupID:   parseInt(row["client_channel_group_id"]),
		})
	}

	sort.Slice(clients, func(i, j int) bool {
		if clients[i].ChannelID != clients[j].ChannelID {
			return clients[i].ChannelID < clients[j].ChannelID
		}
		// talk power 高的排前面
		if clients[i].TalkPower != clients[j].TalkPower {
			return clients[i].TalkPower > clients[j].TalkPower
		}
		// 同 talk power 下按服务器组 sortid 升序
		if clients[i].GroupSortID != clients[j].GroupSortID {
			return clients[i].GroupSortID < clients[j].GroupSortID
		}
		return strings.ToLower(clients[i].Nickname) < strings.ToLower(clients[j].Nickname)
	})

	return clients
}

// resolveGroupPerms 从客户端所属的服务器组中计算最大 talk_power 和最小 sort_id。
func resolveGroupPerms(serverGroups string, allGroupPerms map[int]groupPerms) (talkPower int, sortID int) {
	if serverGroups == "" {
		return 0, math.MaxInt32
	}
	maxTP := 0
	minSortID := math.MaxInt32
	for _, s := range strings.Split(serverGroups, ",") {
		sgid := parseInt(strings.TrimSpace(s))
		if p, ok := allGroupPerms[sgid]; ok {
			if p.talkPower > maxTP {
				maxTP = p.talkPower
			}
			if p.sortID < minSortID {
				minSortID = p.sortID
			}
		}
	}
	if minSortID == math.MaxInt32 {
		minSortID = 0
	}
	return maxTP, minSortID
}
