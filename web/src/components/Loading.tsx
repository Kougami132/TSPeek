import { Spinner, Text, makeStyles, tokens } from '@fluentui/react-components'

const useStyles = makeStyles({
  container: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: '60vh',
    gap: tokens.spacingVerticalL,
  },
  subtitle: {
    color: tokens.colorNeutralForeground2,
    textAlign: 'center',
    maxWidth: '400px',
  },
})

interface LoadingProps {
  errorMessage?: string
}

export function Loading({ errorMessage }: LoadingProps) {
  const styles = useStyles()

  return (
    <div className={styles.container}>
      <Spinner size="large" label="等待首个快照..." />
      <Text size={300} className={styles.subtitle}>
        {errorMessage || '后端已启动，但轮询器尚未产出可用快照。'}
      </Text>
    </div>
  )
}
