import { Archive, HardDriveDownload, Search, Sparkles } from 'lucide-react'
import { useMemo, useState } from 'react'

import {
  getWorldPlayer,
  getWorldPlayers,
  getWorldStatus,
  refreshWorldSnapshot,
  type WorldPlayer,
} from '@/api/palworld'
import { WorldMap } from '@/components/palworld/WorldMap'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { Input } from '@/components/ui/input'
import { Sheet, SheetBody, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

export default function PlayerArchives() {
  const status = useResource(getWorldStatus, [], { refreshIntervalMs: 60_000 })
  const players = useResource(getWorldPlayers, [], { refreshIntervalMs: 60_000 })
  const [query, setQuery] = useState('')
  const [selectedUID, setSelectedUID] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const { showToast } = useGlobalToast()
  const rows = useMemo(() => players.data?.data ?? [], [players.data])
  const filteredRows = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return rows
    return rows.filter((player) =>
      `${player.nickname} ${player.player_uid} ${player.steam_id} ${player.account_name}`.toLowerCase().includes(needle),
    )
  }, [query, rows])
  const palCount = rows.reduce((total, player) => total + (player.pals?.length ?? 0), 0)

  const refreshSnapshot = async () => {
    setRefreshing(true)
    try {
      const result = await refreshWorldSnapshot()
      showToast('success', result.message)
      await wait(1_200)
      status.refresh()
      players.refresh()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '玩家档案刷新失败')
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <PageShell
      title="玩家档案"
      description="玩家等级、帕鲁、背包和最近存档位置。"
      width="full"
      actions={
        <Button onClick={refreshSnapshot} disabled={refreshing}>
          <HardDriveDownload data-icon="inline-start" />
          {refreshing ? '更新中' : '读取最新备份'}
        </Button>
      }
    >
      <div className="flex flex-col gap-5">
        {status.data && !status.data.upToDate ? (
          <Alert>
            <Archive />
            <AlertTitle>索引正在追赶最新备份</AlertTitle>
            <AlertDescription>
              当前索引 {status.data.snapshot.backupId || '-'}，最新备份 {status.data.latestBackupId || '-'}。
            </AlertDescription>
          </Alert>
        ) : null}

        <PageStatStrip>
          <PageStat label="玩家" value={rows.length || '-'} note="包括离线玩家" />
          <PageStat label="帕鲁" value={palCount || '-'} note="玩家持有总数" />
          <PageStat label="索引备份" value={status.data?.snapshot.backupId || '-'} note={status.data?.snapshot.createdAt || '等待快照'} />
          <PageStat label="自动更新" value={status.data?.autoRefreshSeconds ? `${status.data.autoRefreshSeconds} 秒` : '-'} note={status.data?.upToDate ? '已跟上最新备份' : '等待同步'} />
        </PageStatStrip>

        <PageSurface
          title="全部玩家"
          description={players.data ? `索引 ID：${players.data.meta.snapshotId || '-'}，查询于 ${players.data.meta.observedAt}` : '等待世界索引'}
          actions={
            <div className="relative w-64 max-w-full">
              <Search className="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="搜索昵称 / UID"
                className="pl-8"
              />
            </div>
          }
        >
          {players.error ? (
            <ErrorState message={players.error} onRetry={players.refresh} />
          ) : players.loading && !players.data ? (
            <div className="flex h-48 items-center justify-center"><InlineLoader /></div>
          ) : filteredRows.length === 0 ? (
            <EmptyState title="没有匹配的玩家档案" description="清空搜索条件或读取最新备份后重试。" />
          ) : (
            <div className="overflow-x-auto">
              <Table className="min-w-[980px]">
                <TableHeader>
                  <TableRow>
                    <TableHead>玩家</TableHead>
                    <TableHead>等级</TableHead>
                    <TableHead>生命</TableHead>
                    <TableHead>帕鲁</TableHead>
                    <TableHead>物品槽</TableHead>
                    <TableHead>最近位置</TableHead>
                    <TableHead>最后记录</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredRows.map((player) => (
                    <TableRow key={player.player_uid} className="cursor-pointer" onClick={() => setSelectedUID(player.player_uid)}>
                      <TableCell>
                        <div className="font-medium">{player.nickname || player.account_name || player.player_uid}</div>
                        <div className="font-mono text-xs text-muted-foreground">{player.player_uid}</div>
                      </TableCell>
                      <TableCell>{player.level || '-'}</TableCell>
                      <TableCell>{player.max_hp > 0 ? `${player.hp} / ${player.max_hp}` : player.hp || '-'}</TableCell>
                      <TableCell>{player.pals?.length ?? 0}</TableCell>
                      <TableCell>{countItems(player)}</TableCell>
                      <TableCell className="font-mono text-xs">{formatCoordinate(player.location_x, player.location_y)}</TableCell>
                      <TableCell>{player.save_last_online || player.last_online || '-'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </PageSurface>

        <PlayerDetailSheet uid={selectedUID} onOpenChange={(open) => !open && setSelectedUID(null)} />
      </div>
    </PageShell>
  )
}

function PlayerDetailSheet({ uid, onOpenChange }: { uid: string | null; onOpenChange: (open: boolean) => void }) {
  const detail = useResource(() => uid ? getWorldPlayer(uid) : Promise.reject(new Error('未选择玩家')), [uid])
  const player = detail.data?.data

  return (
    <Sheet open={Boolean(uid)} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[760px] max-w-[96vw] overflow-y-auto">
        <SheetHeader className="border-b">
          <SheetTitle>{player?.nickname || '玩家档案'}</SheetTitle>
          <SheetDescription>{player?.player_uid || uid}</SheetDescription>
        </SheetHeader>
        <SheetBody>
          {detail.error ? (
            <ErrorState message={detail.error} onRetry={detail.refresh} />
          ) : detail.loading && !player ? (
            <InlineLoader />
          ) : player ? (
            <div className="flex flex-col gap-6">
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <MiniStat label="等级" value={player.level} />
                <MiniStat label="帕鲁" value={player.pals?.length ?? 0} />
                <MiniStat label="物品槽" value={countItems(player)} />
                <MiniStat label="建筑" value={player.building_count || 0} />
              </div>

              <section className="flex flex-col gap-3">
                <h3 className="text-sm font-semibold">最近存档位置</h3>
                <WorldMap snapshotPlayers={[player]} showLiveEmptyState={false} />
              </section>

              <section className="flex flex-col gap-3">
                <h3 className="text-sm font-semibold">帕鲁</h3>
                <div className="grid gap-2 sm:grid-cols-2">
                  {(player.pals ?? []).map((pal, index) => (
                    <div key={`${pal.type}-${index}`} className="flex items-center justify-between rounded-md border p-3">
                      <div>
                        <div className="font-medium">{pal.nickname || pal.type}</div>
                        <div className="text-xs text-muted-foreground">{pal.type}</div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary">Lv.{pal.level}</Badge>
                        {pal.is_lucky ? <Sparkles className="size-4 text-primary" /> : null}
                      </div>
                    </div>
                  ))}
                </div>
              </section>

              <section className="flex flex-col gap-3">
                <h3 className="text-sm font-semibold">背包与装备</h3>
                <div className="flex flex-col gap-2">
                  {allItems(player).filter((item) => item.StackCount > 0).map((item, index) => (
                    <div key={`${item.ItemId}-${item.SlotIndex}-${index}`} className="flex items-center justify-between border-b py-2 text-sm">
                      <span className="font-mono text-xs">{item.ItemId}</span>
                      <Badge variant="outline">× {item.StackCount}</Badge>
                    </div>
                  ))}
                </div>
              </section>
            </div>
          ) : null}
        </SheetBody>
      </SheetContent>
    </Sheet>
  )
}

function MiniStat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-md border bg-muted/20 p-3">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate font-medium">{value}</div>
    </div>
  )
}

function allItems(player: WorldPlayer) {
  if (!player.items) return []
  return Object.values(player.items).flatMap((items) => items ?? [])
}

function countItems(player: WorldPlayer) {
  return allItems(player).filter((item) => item.StackCount > 0).length
}

function formatCoordinate(x: number, y: number) {
  if (!x && !y) return '-'
  return `${x.toFixed(0)}, ${y.toFixed(0)}`
}

function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}
