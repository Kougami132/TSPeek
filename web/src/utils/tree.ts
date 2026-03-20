import type { ChannelInfo, ClientInfo } from '../types'

/**
 * 将扁平频道列表按 parent_id 构建为树形结构的 Map。
 * key 是 parent_id，value 是该父级下的子频道列表。
 */
export function buildTree(channels: ChannelInfo[]): Map<number, ChannelInfo[]> {
  const tree = new Map<number, ChannelInfo[]>()
  for (const channel of channels) {
    const pid = channel.parent_id
    if (!tree.has(pid)) tree.set(pid, [])
    tree.get(pid)!.push(channel)
  }
  return tree
}

/**
 * 将客户端列表按 channel_id 分组。
 */
export function groupClients(clients: ClientInfo[]): Map<number, ClientInfo[]> {
  const grouped = new Map<number, ClientInfo[]>()
  for (const client of clients) {
    const cid = client.channel_id
    if (!grouped.has(cid)) grouped.set(cid, [])
    grouped.get(cid)!.push(client)
  }
  // 按昵称排序
  grouped.forEach((list) => {
    list.sort((a, b) => a.nickname.localeCompare(b.nickname))
  })
  return grouped
}

/**
 * 格式化时间戳
 */
export function formatTime(value: string | undefined): string {
  if (!value) return '暂无'
  return new Date(value).toLocaleString()
}

/**
 * 格式化运行时间（秒 → 可读字符串）
 */
export function formatUptime(seconds: number): string {
  if (seconds <= 0) return '0 秒'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  const parts: string[] = []
  if (days > 0) parts.push(`${days} 天`)
  if (hours > 0) parts.push(`${hours} 小时`)
  if (mins > 0) parts.push(`${mins} 分钟`)
  if (parts.length === 0) parts.push(`${seconds} 秒`)
  return parts.join(' ')
}
