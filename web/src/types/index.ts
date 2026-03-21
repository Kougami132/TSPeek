// 与 Go 后端 JSON tag 严格一致的类型定义

export interface Snapshot {
  server: ServerInfo
  channels: ChannelInfo[]
  clients: ClientInfo[]
  server_groups: ServerGroupInfo[]
  channel_groups: ChannelGroupInfo[]
  meta: SnapshotMeta
}

export interface SnapshotMeta {
  sequence: number
  fetched_at: string
  latency_ms: number
  stale: boolean
  last_error?: string
}

export interface ServerInfo {
  name: string
  status: string
  platform: string
  version: string
  unique_identifier: string
  channels_online: number
  clients_online: number
  displayed_clients: number
  max_clients: number
  uptime_seconds: number
  created_at: number
}

export interface ChannelInfo {
  id: number
  parent_id: number
  order: number
  name: string
  topic?: string
  total_clients: number
  max_clients: number
  seconds_empty: number
  permanent: boolean
  semi_permanent: boolean
  default: boolean
  password: boolean
}

export interface ClientInfo {
  id: number
  database_id: number
  channel_id: number
  nickname: string
  unique_id?: string
  type: number
  country?: string
  away: boolean
  away_message?: string
  input_muted: boolean
  output_muted: boolean
  output_only_muted: boolean
  input_hardware: boolean
  output_hardware: boolean
  talking: boolean
  talk_power: number
  group_sort_id: number
  idle_seconds: number
  connected_seconds: number
  server_groups?: string
  channel_group_id: number
}

export interface ServerGroupInfo {
  sgid: number
  name: string
  sort_id: number
  icon_id: number
  icon_url?: string
}

export interface ChannelGroupInfo {
  cgid: number
  name: string
  sort_id: number
  icon_id: number
  icon_url?: string
}

export interface PublicConfig {
  server_host: string
  server_port: number
}

export type ConnectionState = 'connecting' | 'live' | 'stale' | 'waiting'
