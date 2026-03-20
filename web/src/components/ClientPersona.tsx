import { Avatar, Badge, makeStyles, tokens } from '@fluentui/react-components'
import {
  MicOffRegular,
  SpeakerMuteRegular,
  PersonArrowRightRegular,
} from '@fluentui/react-icons'
import type { ClientInfo } from '../types'

const useStyles = makeStyles({
  row: {
    display: 'flex',
    alignItems: 'center',
    gap: tokens.spacingHorizontalS,
    paddingTop: tokens.spacingVerticalXS,
    paddingBottom: tokens.spacingVerticalXS,
    paddingLeft: tokens.spacingHorizontalS,
    borderRadius: tokens.borderRadiusMedium,
    ':hover': {
      backgroundColor: tokens.colorNeutralBackground3,
    },
  },
  name: {
    fontWeight: tokens.fontWeightMedium,
    fontSize: tokens.fontSizeBase300,
    flex: 1,
    minWidth: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  icons: {
    display: 'flex',
    gap: tokens.spacingHorizontalXS,
    alignItems: 'center',
    color: tokens.colorNeutralForeground2,
    flexShrink: 0,
  },
})

interface ClientPersonaProps {
  client: ClientInfo
}

export function ClientPersona({ client }: ClientPersonaProps) {
  const styles = useStyles()

  // 获取初始字母作为头像
  const initials = client.nickname.charAt(0).toUpperCase()

  return (
    <div className={styles.row}>
      <Avatar
        name={client.nickname}
        initials={initials}
        size={24}
        color="colorful"
        badge={
          client.away
            ? { status: 'away' }
            : { status: 'available' }
        }
      />
      <span className={styles.name}>{client.nickname}</span>
      <span className={styles.icons}>
        {client.input_muted && <MicOffRegular fontSize={14} />}
        {client.output_muted && <SpeakerMuteRegular fontSize={14} />}
        {client.away && client.away_message && (
          <Badge size="small" appearance="tint" color="warning">
            {client.away_message}
          </Badge>
        )}
        {client.away && !client.away_message && (
          <PersonArrowRightRegular fontSize={14} />
        )}
        {client.country && (
          <Badge size="small" appearance="tint" color="informative">
            {client.country}
          </Badge>
        )}
      </span>
    </div>
  )
}
