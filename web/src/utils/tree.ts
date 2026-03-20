import type { ChannelInfo, ClientInfo } from '../types'

/**
 * 将扁平频道列表按 parent_id 构建为树形结构的 Map，
 * 并按 TeamSpeak 链表顺序（order 字段为前驱频道 ID）排序。
 */
export function buildTree(channels: ChannelInfo[]): Map<number, ChannelInfo[]> {
  const tree = new Map<number, ChannelInfo[]>()
  for (const channel of channels) {
    const pid = channel.parent_id
    if (!tree.has(pid)) tree.set(pid, [])
    tree.get(pid)!.push(channel)
  }

  // 按链表顺序排序每组子频道
  tree.forEach((children) => {
    const byOrder = new Map<number, ChannelInfo>()
    for (const ch of children) byOrder.set(ch.order, ch)

    const sorted: ChannelInfo[] = []
    const visited = new Set<number>()

    const head = byOrder.get(0)
    if (head) {
      sorted.push(head)
      visited.add(head.id)
      let current = head
      for (;;) {
        const next = byOrder.get(current.id)
        if (!next || visited.has(next.id)) break
        sorted.push(next)
        visited.add(next.id)
        current = next
      }
    }

    // 容错：未被遍历到的频道追加到末尾
    for (const ch of children) {
      if (!visited.has(ch.id)) sorted.push(ch)
    }

    children.length = 0
    children.push(...sorted)
  })

  return tree
}

/**
 * 将客户端列表按 channel_id 分组，组内按 talk_power 降序、服务器组 sortid 升序、昵称升序排序。
 */
export function groupClients(clients: ClientInfo[]): Map<number, ClientInfo[]> {
  const grouped = new Map<number, ClientInfo[]>()
  for (const client of clients) {
    const cid = client.channel_id
    if (!grouped.has(cid)) grouped.set(cid, [])
    grouped.get(cid)!.push(client)
  }
  grouped.forEach((list) => {
    list.sort((a, b) => {
      if (a.talk_power !== b.talk_power) return b.talk_power - a.talk_power
      if (a.group_sort_id !== b.group_sort_id) return a.group_sort_id - b.group_sort_id
      return a.nickname.localeCompare(b.nickname)
    })
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
