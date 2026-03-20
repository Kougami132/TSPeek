import {
  Card,
  CardHeader,
  Text,
  makeStyles,
  tokens,
} from '@fluentui/react-components'
import {
  ServerRegular,
  ClockRegular,
  PeopleRegular,
  GlobeRegular,
  InfoRegular,
  TagRegular,
} from '@fluentui/react-icons'
import type { ServerInfo, SnapshotMeta, PublicConfig } from '../types'
import { formatUptime } from '../utils/tree'

const useStyles = makeStyles({
  card: {
    maxWidth: '100%',
  },
  header: {
    paddingBottom: tokens.spacingVerticalXS,
  },
  statsList: {
    display: 'flex',
    flexDirection: 'column',
    gap: tokens.spacingVerticalS,
  },
  statItem: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: tokens.spacingHorizontalS,
  },
  statLabel: {
    color: tokens.colorNeutralForeground2,
    fontSize: tokens.fontSizeBase200,
    display: 'flex',
    alignItems: 'center',
    gap: '4px',
    flexShrink: 0,
  },
  statValue: {
    fontSize: tokens.fontSizeBase300,
    fontWeight: tokens.fontWeightSemibold,
    textAlign: 'right',
    wordBreak: 'break-all',
  },
})

interface ServerCardProps {
  server: ServerInfo
  meta: SnapshotMeta
  publicConfig: PublicConfig
  channelCount: number
}

export function ServerCard({
  server,
  publicConfig,
}: ServerCardProps) {
  const styles = useStyles()

  const address = publicConfig.server_host || '—'

  return (
    <Card className={styles.card} size="large">
      <CardHeader
        className={styles.header}
        image={<ServerRegular fontSize={20} />}
        header={
          <Text weight="bold" size={400}>
            服务器信息
          </Text>
        }
      />

      <div className={styles.statsList}>
        <div className={styles.statItem}>
          <span className={styles.statLabel}>
            <TagRegular fontSize={14} />
            名称
          </span>
          <span className={styles.statValue}>
            {server.name || '未命名'}
          </span>
        </div>
        <div className={styles.statItem}>
          <span className={styles.statLabel}>
            <GlobeRegular fontSize={14} />
            地址
          </span>
          <span className={styles.statValue}>{address}</span>
        </div>
        <div className={styles.statItem}>
          <span className={styles.statLabel}>
            <PeopleRegular fontSize={14} />
            在线用户
          </span>
          <span className={styles.statValue}>
            {server.clients_online} / {server.max_clients}
          </span>
        </div>
        <div className={styles.statItem}>
          <span className={styles.statLabel}>
            <ClockRegular fontSize={14} />
            运行时间
          </span>
          <span className={styles.statValue}>{formatUptime(server.uptime_seconds)}</span>
        </div>
        <div className={styles.statItem}>
          <span className={styles.statLabel}>
            <InfoRegular fontSize={14} />
            版本
          </span>
          <span className={styles.statValue}>{server.version || '未知'}</span>
        </div>
      </div>
    </Card>
  )
}
