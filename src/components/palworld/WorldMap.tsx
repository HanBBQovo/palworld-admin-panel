import { Boxes, MapPin, Users } from 'lucide-react'

import { type Player, type WorldGuild, type WorldPlayer } from '@/api/palworld'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface WorldMapProps {
  livePlayers?: Player[]
  snapshotPlayers?: WorldPlayer[]
  guilds?: WorldGuild[]
  showSnapshotPlayers?: boolean
  showLiveEmptyState?: boolean
}

type MarkerKind = 'live' | 'snapshot' | 'base'

export function WorldMap({
  livePlayers = [],
  snapshotPlayers = [],
  guilds = [],
  showSnapshotPlayers = true,
  showLiveEmptyState = true,
}: WorldMapProps) {
  const liveMarkers = livePlayers.filter(hasLiveLocation)
  const liveUIDs = new Set(liveMarkers.map((player) => player.playerUid).filter(Boolean))
  const savedMarkers = showSnapshotPlayers
    ? snapshotPlayers.filter((player) => hasSnapshotLocation(player) && !liveUIDs.has(player.player_uid))
    : []
  const bases = guilds.flatMap((guild) =>
    (guild.base_camp ?? []).map((base) => ({ guild: guild.name || '未命名公会', base })),
  )

  return (
    <div className="bg-muted p-0 sm:p-3">
      <div className="relative mx-auto aspect-square w-full max-w-[960px] overflow-hidden border bg-background">
        <img
          src="/palworld-map/full-map-z4.png"
          alt="Palworld 世界地图"
          className="absolute inset-0 size-full object-cover"
        />
        <div className="absolute inset-0">
          {bases.map(({ guild, base }) => (
            <MapMarker
              key={`base-${base.id}`}
              x={base.location_x}
              y={base.location_y}
              label={guild}
              detail={`基地坐标 ${formatCoordinate(base.location_x, base.location_y)}`}
              kind="base"
            />
          ))}
          {savedMarkers.map((player) => (
            <MapMarker
              key={`snapshot-${player.player_uid}`}
              x={player.location_x}
              y={player.location_y}
              label={player.nickname || player.player_uid}
              detail="最近存档位置"
              kind="snapshot"
            />
          ))}
          {liveMarkers.map((player) => (
            <MapMarker
              key={`live-${player.id}`}
              x={player.locationX ?? 0}
              y={player.locationY ?? 0}
              label={player.name}
              detail="在线实时位置"
              kind="live"
            />
          ))}
        </div>
        <div className="absolute left-3 top-3 flex flex-wrap gap-2">
          <Badge>在线 {liveMarkers.length}</Badge>
          {showSnapshotPlayers ? <Badge variant="secondary">最近存档 {savedMarkers.length}</Badge> : null}
          <Badge variant="outline">基地 {bases.length}</Badge>
        </div>
        {showLiveEmptyState && liveMarkers.length === 0 ? (
          <div className="absolute bottom-3 left-3 rounded-md border bg-background/95 px-3 py-2 text-xs text-muted-foreground shadow-sm">
            当前没有可显示的在线坐标
          </div>
        ) : null}
      </div>
    </div>
  )
}

function MapMarker({
  x,
  y,
  label,
  detail,
  kind,
}: {
  x: number
  y: number
  label: string
  detail: string
  kind: MarkerKind
}) {
  const position = toScreenPercent(x, y)
  const Icon = kind === 'live' ? Users : kind === 'base' ? Boxes : MapPin
  return (
    <div className="group absolute -translate-x-1/2 -translate-y-1/2" style={position}>
      <span
        className={cn(
          'flex size-7 items-center justify-center rounded-full border-2 shadow-md',
          kind === 'live' && 'border-background bg-primary text-primary-foreground',
          kind === 'snapshot' && 'border-muted-foreground bg-background text-muted-foreground',
          kind === 'base' && 'border-background bg-secondary text-secondary-foreground',
        )}
      >
        <Icon className="size-3.5" />
      </span>
      <span className="pointer-events-none absolute left-1/2 top-8 hidden min-w-max -translate-x-1/2 rounded-md border bg-popover px-2 py-1 text-xs text-popover-foreground shadow-md group-hover:block">
        <span className="block font-medium">{label}</span>
        <span className="block text-muted-foreground">{detail}</span>
      </span>
    </div>
  )
}

function toScreenPercent(worldX: number, worldY: number) {
  const landscape = [447900, 708920, -999940, -738920]
  const normalized = worldX >= -256 && worldX <= 256 && worldY >= -256 && worldY <= 256
  const mapX = normalized ? worldX : -256 + (256 * (worldX - landscape[2])) / (landscape[0] - landscape[2])
  const mapY = normalized ? worldY : (256 * (worldY - landscape[3])) / (landscape[1] - landscape[3])
  const left = clamp((mapY / 256) * 100)
  const top = clamp((-mapX / 256) * 100)
  return { left: `${left}%`, top: `${top}%` }
}

function clamp(value: number) {
  if (!Number.isFinite(value)) return 50
  return Math.max(0, Math.min(100, value))
}

function hasLiveLocation(player: Player) {
  return Number.isFinite(player.locationX) && Number.isFinite(player.locationY) && Boolean(player.locationX || player.locationY)
}

function hasSnapshotLocation(player: WorldPlayer) {
  return Number.isFinite(player.location_x) && Number.isFinite(player.location_y) && Boolean(player.location_x || player.location_y)
}

function formatCoordinate(x: number, y: number) {
  return `${x.toFixed(0)}, ${y.toFixed(0)}`
}
