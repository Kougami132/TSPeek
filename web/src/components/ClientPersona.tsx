import { useMemo, useState, useCallback } from 'react'
import { Avatar, Badge, Tooltip, makeStyles, tokens } from '@fluentui/react-components'
import {
  MicOffRegular,
  SpeakerMuteRegular,
  PersonArrowRightRegular,
} from '@fluentui/react-icons'
import type { ClientInfo, ServerGroupInfo, ChannelGroupInfo } from '../types'

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
  groupIcons: {
    display: 'flex',
    gap: '2px',
    alignItems: 'center',
    flexShrink: 0,
  },
  groupIcon: {
    width: '16px',
    height: '16px',
    objectFit: 'contain',
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
  serverGroupMap: Map<number, ServerGroupInfo>
  channelGroupMap: Map<number, ChannelGroupInfo>
}

export function ClientPersona({ client, serverGroupMap, channelGroupMap }: ClientPersonaProps) {
  const styles = useStyles()

  const initials = client.nickname.charAt(0).toUpperCase()

  // Resolve server group icons (only those with icon_url)
  const serverGroupIcons = useMemo(() => {
    if (!client.server_groups) return []
    return client.server_groups
      .split(',')
      .map((s) => parseInt(s.trim(), 10))
      .map((sgid) => serverGroupMap.get(sgid))
      .filter((sg): sg is ServerGroupInfo => !!sg && !!sg.icon_url)
      .sort((a, b) => a.sort_id - b.sort_id)
  }, [client.server_groups, serverGroupMap])

  // Resolve channel group icon
  const channelGroup = channelGroupMap.get(client.channel_group_id)
  const hasChannelGroupIcon = !!channelGroup?.icon_url

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

      {(serverGroupIcons.length > 0 || hasChannelGroupIcon) && (
        <span className={styles.groupIcons}>
          {serverGroupIcons.map((sg) => (
            <GroupIcon key={sg.sgid} name={sg.name} url={sg.icon_url!} className={styles.groupIcon} />
          ))}
          {hasChannelGroupIcon && (
            <GroupIcon name={channelGroup!.name} url={channelGroup!.icon_url!} className={styles.groupIcon} />
          )}
        </span>
      )}

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

function GroupIcon({ name, url, className }: { name: string; url: string; className: string }) {
  const [visible, setVisible] = useState(true)
  const handleError = useCallback(() => setVisible(false), [])

  if (!visible) return null

  return (
    <Tooltip content={name} relationship="label">
      <img
        src={url}
        alt={name}
        className={className}
        onError={handleError}
      />
    </Tooltip>
  )
}
