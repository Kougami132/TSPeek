package tsquery

import (
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

	sort.Slice(channels, func(i, j int) bool {
		if channels[i].ParentID != channels[j].ParentID {
			return channels[i].ParentID < channels[j].ParentID
		}
		if channels[i].Order != channels[j].Order {
			return channels[i].Order < channels[j].Order
		}
		return channels[i].Name < channels[j].Name
	})

	return channels
}

func mapClients(rows []map[string]string, showQueryClients bool) []store.ClientInfo {
	clients := make([]store.ClientInfo, 0, len(rows))
	for _, row := range rows {
		clientType := parseInt(row["client_type"])
		if !showQueryClients && clientType == 1 {
			continue
		}

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
		return strings.ToLower(clients[i].Nickname) < strings.ToLower(clients[j].Nickname)
	})

	return clients
}
