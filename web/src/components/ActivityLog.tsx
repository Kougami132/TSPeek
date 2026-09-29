import { memo, useState, useCallback } from 'react'
import {
  Card,
  Text,
  makeStyles,
  tokens,
  Badge,
  Button,
  Tooltip,
  Spinner,
} from '@fluentui/react-components'
import {
  PersonAddRegular,
  PersonDeleteRegular,
  ArrowSwapRegular,
  EditRegular,
  ChevronLeftRegular,
  ChevronRightRegular,
  CopyRegular,
  CheckmarkRegular,
  ArrowUpRegular,
  HistoryRegular,
} from '@fluentui/react-icons'
import type { ActivityEvent, ActivityAction } from '../types'
import { useActivities } from '../hooks/useActivities'

const useStyles = makeStyles({
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: tokens.spacingVerticalM,
    width: '100%',
  },
  card: {
    maxWidth: '100%',
    padding: tokens.spacingHorizontalM,
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: tokens.spacingVerticalS,
  },
  headerLeft: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalS,
  },
  titleIcon: {
    color: tokens.colorBrandForeground1,
  },
  unseenBanner: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: tokens.colorBrandBackground2,
    border: `1px solid ${tokens.colorBrandStroke1}`,
    borderRadius: tokens.borderRadiusMedium,
    padding: `${tokens.spacingVerticalXS} ${tokens.spacingHorizontalM}`,
    marginBottom: tokens.spacingVerticalS,
  },
  tableContainer: {
    overflowX: 'auto',
    width: '100%',
  },
  table: {
    width: '100%',
    borderCollapse: 'collapse',
    textAlign: 'left',
    fontSize: tokens.fontSizeBase300,
  },
  th: {
    padding: `${tokens.spacingVerticalS} ${tokens.spacingHorizontalS}`,
    borderBottom: `2px solid ${tokens.colorNeutralStroke2}`,
    color: tokens.colorNeutralForeground2,
    fontWeight: tokens.fontWeightSemibold,
    whiteSpace: 'nowrap',
  },
  tr: {
    borderBottom: `1px solid ${tokens.colorNeutralStroke3}`,
    transitionProperty: 'background-color',
    transitionDuration: '0.15s',
    ':hover': {
      backgroundColor: tokens.colorNeutralBackground1Hover,
    },
  },
  td: {
    padding: `${tokens.spacingVerticalS} ${tokens.spacingHorizontalS}`,
    verticalAlign: 'middle',
  },
  timeCell: {
    whiteSpace: 'nowrap',
    color: tokens.colorNeutralForeground3,
    fontSize: tokens.fontSizeBase200,
    fontVariantNumeric: 'tabular-nums',
  },
  userCell: {
    display: 'flex',
    flexDirection: 'column',
    gap: tokens.spacingVerticalXXS,
  },
  userName: {
    fontWeight: tokens.fontWeightSemibold,
    color: tokens.colorNeutralForeground1,
  },
  uidContainer: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalXXS,
  },
  uidText: {
    fontFamily: tokens.fontFamilyMonospace,
    fontSize: tokens.fontSizeBase100,
    color: tokens.colorNeutralForeground3,
    maxWidth: '180px',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  copyBtn: {
    minWidth: 'auto',
    padding: 0,
    height: '18px',
    width: '18px',
  },
  actionCell: {
    whiteSpace: 'nowrap',
  },
  detailsCell: {
    color: tokens.colorNeutralForeground1,
  },
  channelBadge: {
    backgroundColor: tokens.colorNeutralBackground3,
    color: tokens.colorNeutralForeground1,
    padding: `${tokens.spacingVerticalXXS} ${tokens.spacingHorizontalXS}`,
    borderRadius: tokens.borderRadiusSmall,
    fontSize: tokens.fontSizeBase200,
    fontWeight: tokens.fontWeightMedium,
  },
  movePath: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalXS,
    fontSize: tokens.fontSizeBase200,
  },
  renamePath: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalXS,
    fontSize: tokens.fontSizeBase200,
  },
  emptyState: {
    padding: tokens.spacingVerticalXXL,
    textAlign: 'center',
    color: tokens.colorNeutralForeground3,
  },
  pagination: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: tokens.spacingHorizontalM,
    marginTop: tokens.spacingVerticalM,
    paddingTop: tokens.spacingVerticalS,
    borderTop: `1px solid ${tokens.colorNeutralStroke3}`,
  },
  pageControls: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalXS,
  },
  pageNumBtn: {
    minWidth: '32px',
    height: '32px',
    padding: `0 ${tokens.spacingHorizontalS}`,
  },
  loadingOverlay: {
    display: 'flex',
    justifyContent: 'center',
    padding: tokens.spacingVerticalXXL,
  },
})

function formatDateTime(isoString: string): string {
  try {
    const d = new Date(isoString)
    if (isNaN(d.getTime())) return isoString
    const pad = (n: number) => n.toString().padStart(2, '0')
    const year = d.getFullYear()
    const month = pad(d.getMonth() + 1)
    const day = pad(d.getDate())
    const hours = pad(d.getHours())
    const minutes = pad(d.getMinutes())
    const seconds = pad(d.getSeconds())
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  } catch {
    return isoString
  }
}

function ActionBadge({ action }: { action: ActivityAction }) {
  switch (action) {
    case 'join':
      return (
        <Badge
          appearance="tint"
          color="success"
          icon={<PersonAddRegular />}
        >
          加入
        </Badge>
      )
    case 'leave':
      return (
        <Badge
          appearance="tint"
          color="danger"
          icon={<PersonDeleteRegular />}
        >
          离开
        </Badge>
      )
    case 'move':
      return (
        <Badge
          appearance="tint"
          color="informative"
          icon={<ArrowSwapRegular />}
        >
          切换
        </Badge>
      )
    case 'rename':
      return (
        <Badge
          appearance="tint"
          color="warning"
          icon={<EditRegular />}
        >
          更名
        </Badge>
      )
    default:
      return <Badge appearance="tint">{action}</Badge>
  }
}

function CopyableUID({ uid }: { uid: string }) {
  const styles = useStyles()
  const [copied, setCopied] = useState(false)

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(uid).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }, [uid])

  return (
    <div className={styles.uidContainer}>
      <Tooltip content={copied ? '已复制' : `点击复制 UID: ${uid}`} relationship="description">
        <span className={styles.uidText}>{uid}</span>
      </Tooltip>
      <Tooltip content={copied ? '已复制' : '复制 UID'} relationship="description">
        <Button
          appearance="subtle"
          size="small"
          className={styles.copyBtn}
          icon={copied ? <CheckmarkRegular /> : <CopyRegular />}
          onClick={handleCopy}
        />
      </Tooltip>
    </div>
  )
}

function EventDetails({ event }: { event: ActivityEvent }) {
  const styles = useStyles()

  switch (event.action) {
    case 'join':
      return (
        <span>
          进入频道{' '}
          <span className={styles.channelBadge}>
            {event.channel_name || `#${event.channel_id}`}
          </span>
        </span>
      )
    case 'leave':
      return (
        <span>
          离开频道{' '}
          <span className={styles.channelBadge}>
            {event.channel_name || `#${event.channel_id}`}
          </span>
        </span>
      )
    case 'move':
      return (
        <span className={styles.movePath}>
          <span className={styles.channelBadge}>
            {event.from_channel_name || `#${event.from_channel_id}`}
          </span>
          <span>→</span>
          <span className={styles.channelBadge}>
            {event.channel_name || `#${event.channel_id}`}
          </span>
        </span>
      )
    case 'rename':
      return (
        <span className={styles.renamePath}>
          <Text weight="medium">{event.nickname}</Text>
          <span>→</span>
          <Text weight="semibold" color="brand">{event.target_nickname}</Text>
          {event.channel_name && (
            <span className={styles.channelBadge}>@{event.channel_name}</span>
          )}
        </span>
      )
    default:
      return <span>-</span>
  }
}

export const ActivityLog = memo(function ActivityLog() {
  const styles = useStyles()
  const {
    items,
    page,
    total,
    totalPages,
    loading,
    unseenCount,
    goToPage,
  } = useActivities(50)

  // 计算显示的页码按钮范围（当前页附近最多展示 5 个页码）
  const getPageNumbers = () => {
    if (totalPages <= 1) return []
    const range: number[] = []
    const start = Math.max(1, page - 2)
    const end = Math.min(totalPages, page + 2)
    for (let i = start; i <= end; i++) {
      range.push(i)
    }
    return range
  }

  return (
    <div className={styles.container}>
      <Card className={styles.card}>
        <div className={styles.header}>
          <div className={styles.headerLeft}>
            <HistoryRegular fontSize={24} className={styles.titleIcon} />
            <Text size={500} weight="semibold">
              活动记录
            </Text>
          </div>
          <Text size={200} color="neutral">
            共 {total} 条记录
          </Text>
        </div>

        {unseenCount > 0 && page > 1 && (
          <div className={styles.unseenBanner}>
            <Text size={300} weight="medium">
              收到 {unseenCount} 条新的活动记录
            </Text>
            <Button
              appearance="primary"
              size="small"
              icon={<ArrowUpRegular />}
              onClick={() => goToPage(1)}
            >
              返回最新记录
            </Button>
          </div>
        )}

        {loading && items.length === 0 ? (
          <div className={styles.loadingOverlay}>
            <Spinner label="正在读取活动记录..." />
          </div>
        ) : items.length === 0 ? (
          <div className={styles.emptyState}>
            <Text size={400}>暂无活动记录</Text>
          </div>
        ) : (
          <div className={styles.tableContainer}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th className={styles.th}>时间</th>
                  <th className={styles.th}>操作</th>
                  <th className={styles.th}>玩家</th>
                  <th className={styles.th}>变动详情</th>
                </tr>
              </thead>
              <tbody>
                {items.map((event) => (
                  <tr key={event.id} className={styles.tr}>
                    <td className={`${styles.td} ${styles.timeCell}`}>
                      {formatDateTime(event.time)}
                    </td>
                    <td className={`${styles.td} ${styles.actionCell}`}>
                      <ActionBadge action={event.action} />
                    </td>
                    <td className={`${styles.td} ${styles.userCell}`}>
                      <span className={styles.userName}>{event.nickname}</span>
                      <CopyableUID uid={event.uid} />
                    </td>
                    <td className={`${styles.td} ${styles.detailsCell}`}>
                      <EventDetails event={event} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {totalPages > 1 && (
          <div className={styles.pagination}>
            <Text size={200} color="neutral">
              第 {page} / {totalPages} 页
            </Text>
            <div className={styles.pageControls}>
              <Button
                appearance="subtle"
                size="small"
                icon={<ChevronLeftRegular />}
                disabled={page <= 1 || loading}
                onClick={() => goToPage(page - 1)}
              >
                上一页
              </Button>
              {getPageNumbers().map((num) => (
                <Button
                  key={num}
                  className={styles.pageNumBtn}
                  appearance={num === page ? 'primary' : 'subtle'}
                  size="small"
                  disabled={loading}
                  onClick={() => goToPage(num)}
                >
                  {num}
                </Button>
              ))}
              <Button
                appearance="subtle"
                size="small"
                icon={<ChevronRightRegular />}
                iconPosition="after"
                disabled={page >= totalPages || loading}
                onClick={() => goToPage(page + 1)}
              >
                下一页
              </Button>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
})
