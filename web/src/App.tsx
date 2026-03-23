import { useEffect } from 'react'
import {
  FluentProvider,
  makeStyles,
  tokens,
  Button,
  Text,
  Card,
} from '@fluentui/react-components'
import { PlugConnectedRegular, ClockRegular } from '@fluentui/react-icons'
import { tsPeekTheme } from './theme'
import { useSnapshot } from './hooks/useSnapshot'
import { ServerCard } from './components/ServerCard'
import { ChannelTree } from './components/ChannelTree'
import { Loading } from './components/Loading'
import type { PublicConfig } from './types'
import { formatTime } from './utils/tree'

const HEADER_HEIGHT = '48px'

const useStyles = makeStyles({
  root: {
    minHeight: '100vh',
    backgroundColor: tokens.colorNeutralBackground2,
    display: 'flex',
    flexDirection: 'column',
  },
  header: {
    height: HEADER_HEIGHT,
    backgroundColor: tokens.colorNeutralBackground1,
    borderBottom: `1px solid ${tokens.colorNeutralStroke2}`,
    paddingLeft: tokens.spacingHorizontalL,
    paddingRight: tokens.spacingHorizontalL,
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    position: 'sticky',
    top: '0',
    zIndex: 100,
  },
  headerContent: {
    maxWidth: '1200px',
    width: '100%',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  logoContainer: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalS,
  },
  logo: {
    width: '32px',
    height: '32px',
  },
  shell: {
    maxWidth: '1200px',
    width: '100%',
    marginLeft: 'auto',
    marginRight: 'auto',
    boxSizing: 'border-box',
    display: 'grid',
    gridTemplateColumns: '1fr',
    gap: tokens.spacingHorizontalL,
    padding: tokens.spacingHorizontalL,
    '@media (min-width: 768px)': {
      gridTemplateColumns: '3fr 1fr',
      padding: `${tokens.spacingVerticalL} 0`,
    },
  },
  sidebar: {
    display: 'flex',
    flexDirection: 'column',
    gap: tokens.spacingVerticalL,
    '@media (min-width: 768px)': {
      position: 'sticky',
      top: `calc(${HEADER_HEIGHT} + ${tokens.spacingVerticalL})`,
      alignSelf: 'start',
    },
  },
  updateCard: {
    maxWidth: '100%',
  },
  updateContent: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalXS,
    color: tokens.colorNeutralForeground2,
    fontSize: tokens.fontSizeBase200,
  },
  loadingShell: {
    maxWidth: '1200px',
    width: '100%',
    marginLeft: 'auto',
    marginRight: 'auto',
  },
})

function Header({ publicConfig }: { publicConfig: PublicConfig }) {
  const styles = useStyles()
  const { branding } = publicConfig

  const host = publicConfig.server_host
  const port = publicConfig.server_port
  const joinUrl = host
    ? `ts3server://${host}${port ? `?port=${port}` : ''}`
    : undefined

  const logoSrc = branding.logo_url || '/favicon.svg'
  const title = branding.header_title

  return (
    <header className={styles.header}>
      <div className={styles.headerContent}>
        <div className={styles.logoContainer}>
          <img src={logoSrc} alt={title} className={styles.logo} />
          <Text size={500} weight="bold">
            {title}
          </Text>
        </div>
        {joinUrl ? (
          <Button
            as="a"
            appearance="primary"
            icon={<PlugConnectedRegular />}
            href={joinUrl}
          >
            加入频道
          </Button>
        ) : (
          <Button
            appearance="primary"
            icon={<PlugConnectedRegular />}
            disabled
          >
            加入频道
          </Button>
        )}
      </div>
    </header>
  )
}

function Dashboard() {
  const styles = useStyles()
  const { snapshot, errorMessage, publicConfig } = useSnapshot()

  // 动态更新页面标题
  useEffect(() => {
    document.title = publicConfig.branding.site_title
  }, [publicConfig.branding.site_title])

  // 动态更新 favicon
  useEffect(() => {
    const faviconUrl = publicConfig.branding.favicon_url
    if (!faviconUrl) return

    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      document.head.appendChild(link)
    }
    link.href = faviconUrl
  }, [publicConfig.branding.favicon_url])

  if (!snapshot) {
    return (
      <>
        <Header publicConfig={publicConfig} />
        <div className={styles.loadingShell}>
          <Loading errorMessage={errorMessage} />
        </div>
      </>
    )
  }

  const channels = snapshot.channels ?? []
  const clients = snapshot.clients ?? []
  const serverGroups = snapshot.server_groups ?? []
  const channelGroups = snapshot.channel_groups ?? []

  return (
    <>
      <Header publicConfig={publicConfig} />
      <main className={styles.shell}>
        <ChannelTree
          channels={channels}
          clients={clients}
          serverGroups={serverGroups}
          channelGroups={channelGroups}
        />
        <div className={styles.sidebar}>
          <ServerCard
            server={snapshot.server}
            meta={snapshot.meta}
            publicConfig={publicConfig}
            channelCount={channels.length}
          />
          <Card className={styles.updateCard} size="small">
            <span className={styles.updateContent}>
              <ClockRegular fontSize={14} />
              更新于 {formatTime(snapshot.meta.fetched_at)}
            </span>
          </Card>
        </div>
      </main>
    </>
  )
}

export default function App() {
  const styles = useStyles()

  return (
    <FluentProvider theme={tsPeekTheme}>
      <div className={styles.root}>
        <Dashboard />
      </div>
    </FluentProvider>
  )
}
