import { Badge, makeStyles, tokens } from '@fluentui/react-components'
import type { ConnectionState } from '../types'

const useStyles = makeStyles({
  container: {
    display: 'inline-flex',
    gap: tokens.spacingHorizontalXS,
    alignItems: 'center',
  },
  badge: {
    textTransform: 'uppercase',
    fontSize: tokens.fontSizeBase100,
    fontWeight: tokens.fontWeightSemibold,
  },
})

interface ConnectionStatusProps {
  state: ConnectionState
  errorMessage?: string
}

const stateMap: Record<
  ConnectionState,
  { label: string; color: 'success' | 'warning' | 'danger' | 'informative'; appearance: 'filled' | 'tint' }
> = {
  connecting: { label: '连接中', color: 'informative', appearance: 'tint' },
  live: { label: '实时', color: 'success', appearance: 'filled' },
  stale: { label: '陈旧', color: 'warning', appearance: 'filled' },
  waiting: { label: '等待中', color: 'danger', appearance: 'tint' },
}

export function ConnectionStatus({ state, errorMessage }: ConnectionStatusProps) {
  const styles = useStyles()
  const { label, color, appearance } = stateMap[state]

  return (
    <span className={styles.container}>
      <Badge className={styles.badge} color={color} appearance={appearance}>
        {label}
      </Badge>
      {errorMessage && (
        <Badge className={styles.badge} color="danger" appearance="tint">
          {errorMessage}
        </Badge>
      )}
    </span>
  )
}
