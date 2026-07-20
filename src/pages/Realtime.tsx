import { Activity, Clock3, RefreshCw, Server, Users } from 'lucide-react'

import {
  getAdvancedCapabilities,
  getLiveMap,
  getLiveMetrics,
  getLivePlayers,
  getWorldPlayers,
  getWorldStatus,
} from '@/api/palworld'
import { WorldMap } from '@/components/palworld/WorldMap'
import { WorldIndexAlert } from '@/components/palworld/WorldIndexAlert'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useResource } from '@/lib/use-resource'

const LIVE_REFRESH_MS = 5_000

export default function Realtime() {
  const capabilities = useResource(getAdvancedCapabilities, [], { refreshIntervalMs: 10_000 })
  const players = useResource(getLivePlayers, [], { refreshIntervalMs: LIVE_REFRESH_MS })
  const metrics = useResource(getLiveMetrics, [], { refreshIntervalMs: LIVE_REFRESH_MS })
  const map = useResource(getLiveMap, [], { refreshIntervalMs: LIVE_REFRESH_MS })
  const snapshotPlayers = useResource(getWorldPlayers, [], { refreshIntervalMs: 15_000 })
  const worldStatus = useResource(getWorldStatus, [], { refreshIntervalMs: 5_000 })
  const realtime = capabilities.data?.layers.find((layer) => layer.id === 'realtime')
  const livePlayers = players.data?.data ?? []

  const refresh = () => {
    capabilities.refresh()
    players.refresh()
    metrics.refresh()
    map.refresh()
    snapshotPlayers.refresh()
    worldStatus.refresh()
  }

  return (
    <PageShell
      title="实时地图"
      description="在线玩家位置、服务器性能和连接详情。"
      width="full"
      actions={
        <>
          <Badge variant="secondary">每 5 秒更新</Badge>
          <Button variant="outline" onClick={refresh}>
            <RefreshCw data-icon="inline-start" />
            立即刷新
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-5">
        {realtime && realtime.state !== 'ready' ? (
          <Alert>
            <Server />
            <AlertTitle>{realtime.label}尚未生效</AlertTitle>
            <AlertDescription>{realtime.message}。当前地图会保留最近存档位置和基地，但不会冒充在线坐标。</AlertDescription>
          </Alert>
        ) : null}

        <WorldIndexAlert status={worldStatus.data} />

        <PageStatStrip>
          <PageStat label="在线玩家" value={livePlayers.length} note={players.data?.meta.source ?? '等待服务器响应'} />
          <PageStat label="服务器 FPS" value={metrics.data?.serverFps ?? '-'} note={metrics.data?.source ?? '等待 REST 指标'} />
          <PageStat label="帧时间" value={metrics.data ? `${metrics.data.serverFrameTime.toFixed(2)} ms` : '-'} note="实时服务器帧耗时" />
          <PageStat label="运行时长" value={metrics.data ? formatDuration(metrics.data.uptimeSeconds) : '-'} note={players.data?.meta.observedAt ?? '尚未刷新'} />
        </PageStatStrip>

        <PageSurface
          title="世界地图"
          description="实心标记是在线位置，空心标记是最近存档位置。"
          bodyClassName="p-0"
        >
          {map.error && !map.data ? (
            <ErrorState message={map.error} onRetry={map.refresh} />
          ) : map.loading && !map.data ? (
            <div className="flex h-80 items-center justify-center"><InlineLoader /></div>
          ) : (
            <WorldMap
              livePlayers={map.data?.data.players ?? []}
              snapshotPlayers={snapshotPlayers.data?.data ?? []}
              guilds={map.data?.data.guilds ?? []}
            />
          )}
        </PageSurface>

        <PageSurface
          title="在线详情"
          description={players.data ? `数据源：${players.data.meta.source}，更新于 ${players.data.meta.observedAt}` : '等待服务器响应'}
        >
          {players.error ? (
            <ErrorState message={players.error} onRetry={players.refresh} />
          ) : players.loading && !players.data ? (
            <div className="flex h-48 items-center justify-center"><InlineLoader /></div>
          ) : livePlayers.length === 0 ? (
            <EmptyState
              title="当前没有在线玩家"
              description="有人进入服务器后，这里会在 5 秒内显示等级、延迟、坐标和接入地址。"
              icon={<Users className="size-5" />}
              actions={<Button variant="outline" onClick={players.refresh}><RefreshCw data-icon="inline-start" />重新检查</Button>}
            />
          ) : (
            <div className="overflow-x-auto">
              <Table className="min-w-[900px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>玩家</TableHead>
                    <TableHead>等级</TableHead>
                    <TableHead>延迟</TableHead>
                    <TableHead>坐标</TableHead>
                    <TableHead>建筑</TableHead>
                    <TableHead>接入 IP</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {livePlayers.map((player) => (
                    <TableRow key={player.id}>
                      <TableCell>
                        <div className="font-medium">{player.name}</div>
                        <div className="text-xs text-muted-foreground">{player.platform}</div>
                      </TableCell>
                      <TableCell>{player.level || '-'}</TableCell>
                      <TableCell>{player.ping ? `${player.ping.toFixed(0)} ms` : '-'}</TableCell>
                      <TableCell className="font-mono text-xs">{formatLiveCoordinate(player.locationX, player.locationY)}</TableCell>
                      <TableCell>{player.buildingCount || '-'}</TableCell>
                      <TableCell className="font-mono text-xs">{player.ip || '-'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </PageSurface>

        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <Activity className="size-4" />
          <span>页面隐藏时暂停轮询，重新打开后继续更新。</span>
          <Clock3 className="ml-auto size-4" />
          <span>{players.data?.meta.observedAt ?? '-'}</span>
        </div>
      </div>
    </PageShell>
  )
}

function formatDuration(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (hours > 0) return `${hours} 小时 ${minutes} 分`
  return `${minutes} 分钟`
}

function formatLiveCoordinate(x?: number, y?: number) {
  if (!Number.isFinite(x) || !Number.isFinite(y) || (!x && !y)) return '-'
  return `${x?.toFixed(0)}, ${y?.toFixed(0)}`
}
