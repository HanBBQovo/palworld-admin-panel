import { Boxes, HardDriveDownload, RefreshCw, Users } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'

import { getWorldGuilds, getWorldStatus, refreshWorldSnapshot, type WorldGuild } from '@/api/palworld'
import { WorldMap } from '@/components/palworld/WorldMap'
import { WorldIndexAlert } from '@/components/palworld/WorldIndexAlert'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

export default function WorldGuilds() {
  const status = useResource(getWorldStatus, [], { refreshIntervalMs: 5_000 })
  const guilds = useResource(getWorldGuilds, [], { refreshIntervalMs: 60_000 })
  const [selectedKey, setSelectedKey] = useState('')
  const [refreshing, setRefreshing] = useState(false)
  const { showToast } = useGlobalToast()
  const rows = useMemo(() => guilds.data?.data ?? [], [guilds.data])
  const selected = useMemo(
    () => rows.find((guild) => guildKey(guild) === selectedKey) ?? rows[0] ?? null,
    [rows, selectedKey],
  )
  const memberCount = rows.reduce((total, guild) => total + (guild.players?.length ?? 0), 0)
  const baseCount = rows.reduce((total, guild) => total + (guild.base_camp?.length ?? 0), 0)

  useEffect(() => {
    if (!selectedKey && rows[0]) setSelectedKey(guildKey(rows[0]))
  }, [rows, selectedKey])

  const refreshSnapshot = async () => {
    setRefreshing(true)
    try {
      const result = await refreshWorldSnapshot()
      showToast('success', result.message)
      status.refresh()
      guilds.refresh()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '世界数据刷新失败')
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <PageShell
      title="世界地图与公会"
      description="查看基地分布、公会成员和最近世界快照。"
      width="full"
      actions={
        <>
          <Button variant="outline" onClick={() => { status.refresh(); guilds.refresh() }}>
            <RefreshCw data-icon="inline-start" />
            刷新页面
          </Button>
          <Button onClick={refreshSnapshot} disabled={refreshing}>
            <HardDriveDownload data-icon="inline-start" />
            {refreshing ? '更新中' : '读取最新备份'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-5">
        <WorldIndexAlert status={status.data} />

        <PageStatStrip>
          <PageStat label="世界快照" value={status.data?.snapshot.backupId || '-'} note={status.data?.indexUpdatedAt || status.data?.snapshot.createdAt || '等待快照'} />
          <PageStat label="公会" value={rows.length || '-'} note="来自 Level.sav" />
          <PageStat label="成员记录" value={memberCount || '-'} note="按公会汇总" />
          <PageStat label="基地" value={baseCount || '-'} note="地图坐标可见" />
        </PageStatStrip>

        <PageSurface title="基地分布" description="点击公会列表后，下方会显示成员和每个基地的准确坐标。" bodyClassName="p-0">
          {guilds.error && !guilds.data ? (
            <ErrorState message={guilds.error} onRetry={guilds.refresh} />
          ) : guilds.loading && !guilds.data ? (
            <div className="flex h-80 items-center justify-center"><InlineLoader /></div>
          ) : (
            <WorldMap guilds={rows} showSnapshotPlayers={false} showLiveEmptyState={false} />
          )}
        </PageSurface>

        <div className="grid gap-5 xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]">
          <PageSurface title="公会列表" description={guilds.data ? `更新于 ${guilds.data.meta.observedAt}` : '等待世界索引'}>
            {guilds.error ? (
              <ErrorState message={guilds.error} onRetry={guilds.refresh} />
            ) : guilds.loading && !guilds.data ? (
              <InlineLoader />
            ) : rows.length === 0 ? (
              <EmptyState title="暂无公会数据" description="读取最新备份后仍为空时，当前世界尚未建立公会。" />
            ) : (
              <div className="overflow-x-auto">
                <Table className="min-w-[680px]">
                  <TableHeader>
                    <TableRow>
                      <TableHead>公会</TableHead>
                      <TableHead>等级</TableHead>
                      <TableHead>成员</TableHead>
                      <TableHead>基地</TableHead>
                      <TableHead>会长 UID</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {rows.map((guild) => (
                      <TableRow
                        key={guildKey(guild)}
                        data-state={selected && guildKey(guild) === guildKey(selected) ? 'selected' : undefined}
                        className="cursor-pointer"
                        onClick={() => setSelectedKey(guildKey(guild))}
                      >
                        <TableCell className="font-medium">{guild.name || '未命名公会'}</TableCell>
                        <TableCell>{guild.base_camp_level || '-'}</TableCell>
                        <TableCell>{guild.players?.length ?? 0}</TableCell>
                        <TableCell>{guild.base_camp?.length ?? 0}</TableCell>
                        <TableCell className="font-mono text-xs">{guild.admin_player_uid || '-'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </PageSurface>

          <PageSurface title={selected?.name || '公会详情'} description={selected ? `基地等级 ${selected.base_camp_level || '-'}` : '从列表选择公会'}>
            {selected ? (
              <div className="flex flex-col gap-5">
                <section className="flex flex-col gap-3">
                  <div className="flex items-center gap-2 text-sm font-semibold"><Users className="size-4" />成员</div>
                  <div className="flex flex-col gap-2">
                    {(selected.players ?? []).map((member) => (
                      <div key={member.player_uid} className="flex items-center justify-between gap-3 border-b py-2 text-sm">
                        <span>{member.nickname || '未命名玩家'}</span>
                        <span className="max-w-[58%] truncate font-mono text-xs text-muted-foreground">{member.player_uid}</span>
                      </div>
                    ))}
                  </div>
                </section>

                <section className="flex flex-col gap-3">
                  <div className="flex items-center gap-2 text-sm font-semibold"><Boxes className="size-4" />基地</div>
                  <div className="flex flex-col gap-2">
                    {(selected.base_camp ?? []).map((base, index) => (
                      <div key={base.id} className="rounded-md border p-3">
                        <div className="flex items-center justify-between gap-3">
                          <span className="font-medium">基地 {index + 1}</span>
                          <Badge variant="outline">区域 {base.area || '-'}</Badge>
                        </div>
                        <div className="mt-2 font-mono text-xs text-muted-foreground">
                          {base.location_x.toFixed(0)}, {base.location_y.toFixed(0)}
                        </div>
                      </div>
                    ))}
                  </div>
                </section>
              </div>
            ) : (
              <EmptyState title="尚未选择公会" />
            )}
          </PageSurface>
        </div>
      </div>
    </PageShell>
  )
}

function guildKey(guild: WorldGuild) {
  return guild.admin_player_uid || guild.name
}
