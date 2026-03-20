import { memo, useMemo, useState, useCallback } from 'react'
import {
  Card,
  CardHeader,
  Text,
  makeStyles,
  tokens,
  Badge,
  mergeClasses,
} from '@fluentui/react-components'
import {
  ChannelRegular,
  LockClosedRegular,
  ChevronRightRegular,
} from '@fluentui/react-icons'
import type { ChannelInfo, ClientInfo } from '../types'
import { buildTree, groupClients } from '../utils/tree'
import { ClientPersona } from './ClientPersona'

const useStyles = makeStyles({
  card: {
    maxWidth: '100%',
  },
  list: {
    listStyleType: 'none',
    padding: 0,
    margin: 0,
  },
  channelStack: {
    display: 'flex',
    flexDirection: 'column',
  },
  channelNode: {
    display: 'flex',
    flexDirection: 'column',
  },
  channelHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalXXS,
    paddingTop: tokens.spacingVerticalXXS,
    paddingBottom: tokens.spacingVerticalXXS,
    paddingLeft: tokens.spacingHorizontalXS,
    paddingRight: tokens.spacingHorizontalXS,
    fontWeight: tokens.fontWeightSemibold,
    fontSize: tokens.fontSizeBase300,
    cursor: 'pointer',
    borderRadius: tokens.borderRadiusMedium,
    transitionProperty: 'background-color',
    transitionDuration: '0.1s',
    transitionTimingFunction: 'ease',
    ':hover': {
      backgroundColor: tokens.colorNeutralBackground3,
    },
    userSelect: 'none',
  },
  chevronSlot: {
    width: '16px',
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    flexShrink: 0,
  },
  iconSlot: {
    width: '20px',
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    flexShrink: 0,
  },
  chevronIcon: {
    color: tokens.colorNeutralForeground3,
    transitionProperty: 'transform',
    transitionDuration: '0.2s',
    transitionTimingFunction: 'ease',
  },
  chevronExpanded: {
    transform: 'rotate(90deg)',
  },
  channelIcon: {
    color: tokens.colorBrandForeground1,
  },
  channelName: {
    flex: 1,
    minWidth: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  topic: {
    color: tokens.colorNeutralForeground2,
    fontSize: tokens.fontSizeBase200,
    // 16px(chevron) + 2px(gap) + 20px(icon) + 2px(gap) + 4px(padding) = ~44px
    paddingLeft: 'calc(16px + 20px + 8px)',
    fontStyle: 'italic',
    marginBottom: tokens.spacingVerticalXXS,
  },
  children: {
    paddingLeft: tokens.spacingHorizontalM,
    borderLeft: `1px solid ${tokens.colorNeutralStroke3}`,
    marginLeft: '8px',
  },
  empty: {
    color: tokens.colorNeutralForeground2,
    fontSize: tokens.fontSizeBase200,
    padding: tokens.spacingVerticalM,
    textAlign: 'center',
  },
  clientGroup: {
    paddingLeft: tokens.spacingHorizontalM,
    borderLeft: `1px solid ${tokens.colorNeutralStroke3}`,
    marginLeft: '8px',
  },
})

interface ChannelTreeProps {
  channels: ChannelInfo[]
  clients: ClientInfo[]
}

interface ChannelNodeProps {
  channel: ChannelInfo
  byParent: Map<number, ChannelInfo[]>
  byChannel: Map<number, ClientInfo[]>
  styles: ReturnType<typeof useStyles>
}

const ChannelNode = memo(function ChannelNode({
  channel,
  byParent,
  byChannel,
  styles,
}: ChannelNodeProps) {
  const [isExpanded, setIsExpanded] = useState(true)
  const children = byParent.get(channel.id) || []
  const clients = byChannel.get(channel.id) || []
  const hasContent = children.length > 0 || clients.length > 0

  const toggleExpand = useCallback(() => {
    if (hasContent) {
      setIsExpanded((prev) => !prev)
    }
  }, [hasContent])

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      toggleExpand()
    }
  }, [toggleExpand])

  return (
    <li role="treeitem" aria-expanded={hasContent ? isExpanded : undefined} className={styles.channelNode}>
      <div
        className={styles.channelHeader}
        onClick={toggleExpand}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        role="button"
        aria-label={`${channel.name}${hasContent ? (isExpanded ? '，已展开' : '，已折叠') : ''}`}
      >
        <div className={styles.chevronSlot}>
          {hasContent && (
            <ChevronRightRegular
              className={mergeClasses(styles.chevronIcon, isExpanded && styles.chevronExpanded)}
              fontSize={12}
            />
          )}
        </div>
        <div className={styles.iconSlot}>
          <ChannelRegular className={styles.channelIcon} fontSize={16} />
        </div>
        <span className={styles.channelName}>{channel.name}</span>
        {channel.password && <LockClosedRegular fontSize={14} />}
        {channel.total_clients > 0 && (
          <Badge size="small" appearance="tint" color="brand">
            {channel.total_clients}
          </Badge>
        )}
      </div>

      {isExpanded && (
        <>
          {channel.topic && <div className={styles.topic}>{channel.topic}</div>}
          {clients.length > 0 && (
            <ul role="group" className={mergeClasses(styles.list, styles.clientGroup)}>
              {clients.map((client) => (
                <li key={client.id} role="none">
                  <ClientPersona client={client} />
                </li>
              ))}
            </ul>
          )}
          {children.length > 0 && (
            <ul role="group" className={mergeClasses(styles.list, styles.children)}>
              {children.map((child) => (
                <ChannelNode
                  key={child.id}
                  channel={child}
                  byParent={byParent}
                  byChannel={byChannel}
                  styles={styles}
                />
              ))}
            </ul>
          )}
        </>
      )}
    </li>
  )
})

export function ChannelTree({ channels, clients }: ChannelTreeProps) {
  const styles = useStyles()

  const byParent = useMemo(() => buildTree(channels), [channels])
  const byChannel = useMemo(() => groupClients(clients), [clients])
  const rootChannels = byParent.get(0) || []

  return (
    <Card className={styles.card} size="large">
      <CardHeader
        image={<ChannelRegular fontSize={24} />}
        header={
          <Text weight="bold" size={400}>
            频道列表
          </Text>
        }
      />

      {rootChannels.length > 0 ? (
        <ul role="tree" aria-label="频道列表" className={mergeClasses(styles.list, styles.channelStack)}>
          {rootChannels.map((channel) => (
            <ChannelNode
              key={channel.id}
              channel={channel}
              byParent={byParent}
              byChannel={byChannel}
              styles={styles}
            />
          ))}
        </ul>
      ) : (
        <Text className={styles.empty}>ServerQuery 未返回任何频道。</Text>
      )}
    </Card>
  )
}
