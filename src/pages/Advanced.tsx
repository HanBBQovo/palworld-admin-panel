import { useMemo, useState } from 'react'
import {
  Activity,
  Backpack,
  Boxes,
  Database,
  ExternalLink,
  HardDriveDownload,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Power,
  PowerOff,
  Users,
  Wrench,
} from 'lucide-react'

import {
  createEditorPreview,
  getAdvancedCapabilities,
  getEditorStatus,
  getLiveMap,
  getLiveMetrics,
  getLivePlayers,
  getWorldGuilds,
  getWorldPlayer,
  getWorldPlayers,
  getWorldStatus,
  refreshWorldSnapshot,
  runEditorSession,
  type AdvancedLayer,
  type EditorPreview,
  type Player,
  type WorldGuild,
  type WorldPlayer,
} from '@/api/palworld'
import { InlineLoader } from '@/components/PageLoader'
import {
  PageShell,
  PageStat,
  PageStatStrip,
  PageSubnav,
  PageSubnavButton,
  PageSurface,
} from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Sheet, SheetBody, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'
import { cn } from '@/lib/utils'

type AdvancedTab = 'realtime' | 'players' | 'world' | 'editor'

const layerTone: Record<AdvancedLayer['state'], 'default' | 'secondary' | 'destructive' | 'outline'> = {
  ready: 'default',
  disabled: 'outline',
  'pending-restart': 'secondary',
  degraded: 'destructive',
  'not-installed': 'outline',
  'snapshot-ready': 'secondary',
  locked: 'outline',
}

const layerStateLabel: Record<AdvancedLayer['state'], string> = {
  ready: '已就绪',
  disabled: '未启用',
  'pending-restart': '等待安全重启',
  degraded: '需要检查',
  'not-installed': '尚未安装',
  'snapshot-ready': '快照已准备',
  locked: '维护锁定',
}

export default function Advanced() {
  const [tab, setTab] = useState<AdvancedTab>('realtime')
  const capabilities = useResource(getAdvancedCapabilities, [])
  const worldStatus = useResource(getWorldStatus, [])
  const [refreshing, setRefreshing] = useState(false)
  const { showToast } = useGlobalToast()

  const refreshAll = () => {
    capabilities.refresh()
    worldStatus.refresh()
  }

  const refreshSnapshot = async () => {
    setRefreshing(true)
    try {
      const result = await refreshWorldSnapshot()
      showToast('success', result.message)
      refreshAll()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '世界快照刷新失败')
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <PageShell
      title="高级游戏控制台"
      description="实时管理、世界情报与维护编辑使用统一认证和审计。"
      width="full"
      actions={
        <Button variant="outline" onClick={refreshAll} disabled={capabilities.loading}>
          <RefreshCw data-icon="inline-start" className={capabilities.loading ? 'animate-spin' : undefined} />
          刷新状态
        </Button>
      }
    >
      <div className="flex flex-col gap-5">
        {capabilities.error ? (
          <PageSurface><ErrorState message={capabilities.error} onRetry={capabilities.refresh} /></PageSurface>
        ) : capabilities.loading && !capabilities.data ? (
          <div className="flex h-48 items-center justify-center"><InlineLoader /></div>
        ) : capabilities.data ? (
          <>
            <div className="grid gap-3 xl:grid-cols-3">
              {capabilities.data.layers.map((layer) => <LayerStatus key={layer.id} layer={layer} />)}
            </div>

            {capabilities.data.safety.playersOnline > 0 ? (
              <Alert>
                <ShieldCheck />
                <AlertTitle>在线保护已启用</AlertTitle>
                <AlertDescription>
                  当前 {capabilities.data.safety.playersOnline} 人在线。实时读取正常，游戏重启和世界存档写回保持锁定。
                </AlertDescription>
              </Alert>
            ) : null}

            <PageSubnav>
              <PageSubnavButton active={tab === 'realtime'} onClick={() => setTab('realtime')}>实时态势</PageSubnavButton>
              <PageSubnavButton active={tab === 'players'} onClick={() => setTab('players')}>玩家档案</PageSubnavButton>
              <PageSubnavButton active={tab === 'world'} onClick={() => setTab('world')}>世界与公会</PageSubnavButton>
              <PageSubnavButton active={tab === 'editor'} onClick={() => setTab('editor')}>维护编辑</PageSubnavButton>
            </PageSubnav>

            {tab === 'realtime' ? <RealtimeWorkspace /> : null}
            {tab === 'players' ? <PlayersWorkspace /> : null}
            {tab === 'world' ? <WorldWorkspace worldStatus={worldStatus.data} onRefreshSnapshot={refreshSnapshot} refreshing={refreshing} /> : null}
            {tab === 'editor' ? <EditorWorkspace /> : null}
          </>
        ) : null}
      </div>
    </PageShell>
  )
}

function LayerStatus({ layer }: { layer: AdvancedLayer }) {
  const Icon = layer.id === 'realtime' ? Activity : layer.id === 'world-index' ? Database : Wrench
  return (
    <PageSurface bodyClassName="h-full">
      <div className="flex h-full flex-col gap-4">
        <div className="flex items-start justify-between gap-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-md border bg-muted/40"><Icon className="h-5 w-5" /></span>
          <Badge variant={layerTone[layer.state]}>{layerStateLabel[layer.state]}</Badge>
        </div>
        <div>
          <h2 className="font-semibold">{layer.label}</h2>
          <p className="mt-1 text-sm leading-6 text-muted-foreground">{layer.message}</p>
        </div>
        <div className="mt-auto border-t pt-3 text-xs text-muted-foreground">{layer.source}</div>
      </div>
    </PageSurface>
  )
}

function RealtimeWorkspace() {
  const players = useResource(getLivePlayers, [])
  const metrics = useResource(getLiveMetrics, [])
  const map = useResource(getLiveMap, [])
  const refresh = () => {
    players.refresh()
    metrics.refresh()
    map.refresh()
  }
  const livePlayers = players.data?.data ?? []
  return (
    <div className="flex flex-col gap-5">
      <PageStatStrip>
        <PageStat label="在线玩家" value={livePlayers.length} note={players.data?.meta.source ?? '等待数据'} />
        <PageStat label="服务器 FPS" value={metrics.data?.serverFps ?? '-'} note={metrics.error ? '等待 REST 启用' : '官方实时指标'} />
        <PageStat label="帧时间" value={metrics.data ? `${metrics.data.serverFrameTime.toFixed(2)} ms` : '-'} note="越低越稳定" />
        <PageStat label="游戏天数" value={metrics.data?.inGameDays ?? '-'} note="来自服务器世界状态" />
      </PageStatStrip>

      <PageSurface
        title="实时世界地图"
        description="玩家坐标来自官方 REST；基地坐标来自最近一次世界快照。"
        actions={<Button variant="outline" size="sm" onClick={refresh}><RefreshCw data-icon="inline-start" />刷新</Button>}
        bodyClassName="p-0"
      >
        <WorldMap players={map.data?.data.players ?? []} guilds={map.data?.data.guilds ?? []} restPending={Boolean(map.data?.meta.stale)} />
      </PageSurface>

      <PageSurface title="在线详情" description="REST 启用后显示等级、延迟、IP、坐标和建筑数量。">
        {players.error ? <ErrorState message={players.error} onRetry={players.refresh} /> : players.loading && !players.data ? <InlineLoader /> : (
          <Table className="min-w-[920px]">
            <TableHeader><TableRow><TableHead>玩家</TableHead><TableHead>等级</TableHead><TableHead>延迟</TableHead><TableHead>坐标</TableHead><TableHead>建筑</TableHead><TableHead>接入 IP</TableHead></TableRow></TableHeader>
            <TableBody>
              {livePlayers.map((player) => (
                <TableRow key={player.id}>
                  <TableCell><div className="font-medium">{player.name}</div><div className="text-xs text-muted-foreground">{player.platform}</div></TableCell>
                  <TableCell>{player.level || '-'}</TableCell>
                  <TableCell>{player.ping ? `${player.ping.toFixed(0)} ms` : '-'}</TableCell>
                  <TableCell className="font-mono text-xs">{hasLocation(player) ? `${player.locationX?.toFixed(0)}, ${player.locationY?.toFixed(0)}` : '-'}</TableCell>
                  <TableCell>{player.buildingCount || '-'}</TableCell>
                  <TableCell className="font-mono text-xs">{player.ip || '-'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </PageSurface>
    </div>
  )
}

function WorldMap({ players, guilds, restPending }: { players: Player[]; guilds: WorldGuild[]; restPending: boolean }) {
  const markers = players.filter(hasLocation)
  const bases = guilds.flatMap((guild) => (guild.base_camp ?? []).map((base) => ({ guild: guild.name, base })))
  return (
    <div className="bg-[#10181a] p-0 sm:p-3">
      <div className="relative mx-auto aspect-square w-full max-w-[960px] overflow-hidden">
        <img src="/palworld-map/full-map-z4.png" alt="Palworld 世界地图" className="absolute inset-0 h-full w-full" />
        <div className="absolute inset-0">
          {bases.map(({ guild, base }) => <MapMarker key={base.id} x={base.location_x} y={base.location_y} label={guild || '基地'} kind="base" />)}
          {markers.map((player) => <MapMarker key={player.id} x={player.locationX ?? 0} y={player.locationY ?? 0} label={player.name} kind="player" />)}
        </div>
        <div className="absolute left-3 top-3 flex flex-wrap gap-2">
          <Badge variant="secondary">玩家 {markers.length}</Badge>
          <Badge variant="outline">基地 {bases.length}</Badge>
          {restPending ? <Badge variant="outline">等待 REST 坐标</Badge> : null}
        </div>
      </div>
    </div>
  )
}

function MapMarker({ x, y, label, kind }: { x: number; y: number; label: string; kind: 'player' | 'base' }) {
  const position = toScreenPercent(x, y)
  return (
    <div className="group absolute -translate-x-1/2 -translate-y-1/2" style={position}>
      <span className={cn('flex h-7 w-7 items-center justify-center rounded-full border-2 border-white shadow-md', kind === 'player' ? 'bg-cyan-600' : 'bg-amber-500')}>
        {kind === 'player' ? <Users className="h-3.5 w-3.5 text-white" /> : <Boxes className="h-3.5 w-3.5 text-white" />}
      </span>
      <span className="pointer-events-none absolute left-1/2 top-8 z-10 hidden -translate-x-1/2 whitespace-nowrap rounded-md bg-black/85 px-2 py-1 text-xs text-white group-hover:block">{label}</span>
    </div>
  )
}

function toScreenPercent(worldX: number, worldY: number) {
  const landscape = [447900, 708920, -999940, -738920]
  const normalized = worldX >= -256 && worldX <= 256 && worldY >= -256 && worldY <= 256
  const mapX = normalized ? worldX : -256 + (256 * (worldX - landscape[2])) / (landscape[0] - landscape[2])
  const mapY = normalized ? worldY : (256 * (worldY - landscape[3])) / (landscape[1] - landscape[3])
  return { left: `${(mapY / 256) * 100}%`, top: `${(-mapX / 256) * 100}%` }
}

function hasLocation(player: Player) {
  return Boolean(player.locationX || player.locationY)
}

function PlayersWorkspace() {
  const players = useResource(getWorldPlayers, [])
  const [query, setQuery] = useState('')
  const [selectedUID, setSelectedUID] = useState<string | null>(null)
  const rows = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return players.data?.data ?? []
    return (players.data?.data ?? []).filter((player) => `${player.nickname} ${player.player_uid} ${player.steam_id}`.toLowerCase().includes(needle))
  }, [players.data, query])
  return (
    <>
      <PageSurface
        title="全部玩家档案"
        description="来自完成备份的只读解析，包括离线玩家。"
        actions={<div className="relative w-64 max-w-full"><Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索昵称 / UID" className="pl-8" /></div>}
      >
        {players.error ? <ErrorState message={players.error} onRetry={players.refresh} /> : players.loading && !players.data ? <div className="flex h-48 items-center justify-center"><InlineLoader /></div> : rows.length === 0 ? <EmptyState title="暂无玩家档案" description="先在世界与公会中准备静态快照并等待索引完成。" /> : (
          <Table className="min-w-[900px]">
            <TableHeader><TableRow><TableHead>玩家</TableHead><TableHead>等级</TableHead><TableHead>生命 / 护盾</TableHead><TableHead>帕鲁</TableHead><TableHead>背包槽</TableHead><TableHead>最后记录</TableHead></TableRow></TableHeader>
            <TableBody>{rows.map((player) => (
              <TableRow key={player.player_uid} className="cursor-pointer" onClick={() => setSelectedUID(player.player_uid)}>
                <TableCell><div className="font-medium">{player.nickname || player.account_name || player.player_uid}</div><div className="font-mono text-xs text-muted-foreground">{player.player_uid}</div></TableCell>
                <TableCell>{player.level}</TableCell>
                <TableCell>{player.hp} / {player.max_hp}<div className="text-xs text-muted-foreground">护盾 {player.shield_hp} / {player.shield_max_hp}</div></TableCell>
                <TableCell>{player.pals?.length ?? '-'}</TableCell>
                <TableCell>{countItems(player)}</TableCell>
                <TableCell>{player.save_last_online || player.last_online || '-'}</TableCell>
              </TableRow>
            ))}</TableBody>
          </Table>
        )}
      </PageSurface>
      <PlayerDetailSheet uid={selectedUID} onOpenChange={(open) => !open && setSelectedUID(null)} />
    </>
  )
}

function PlayerDetailSheet({ uid, onOpenChange }: { uid: string | null; onOpenChange: (open: boolean) => void }) {
  const detail = useResource(() => uid ? getWorldPlayer(uid) : Promise.reject(new Error('未选择玩家')), [uid])
  const player = detail.data?.data
  return (
    <Sheet open={Boolean(uid)} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[720px] max-w-[96vw] overflow-y-auto">
        <SheetHeader className="border-b">
          <SheetTitle>{player?.nickname || '玩家档案'}</SheetTitle>
          <SheetDescription>{player?.player_uid || uid}</SheetDescription>
        </SheetHeader>
        <SheetBody>
          {detail.error ? <ErrorState message={detail.error} onRetry={detail.refresh} /> : detail.loading && !player ? <InlineLoader /> : player ? (
            <div className="space-y-6">
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <MiniStat label="等级" value={player.level} />
                <MiniStat label="帕鲁" value={player.pals?.length ?? 0} />
                <MiniStat label="背包物品" value={countItems(player)} />
                <MiniStat label="建筑" value={player.building_count || 0} />
              </div>
              <section>
                <h3 className="mb-3 text-sm font-semibold">帕鲁</h3>
                <div className="grid gap-2 sm:grid-cols-2">{(player.pals ?? []).slice(0, 60).map((pal, index) => (
                  <div key={`${pal.type}-${index}`} className="flex items-center justify-between rounded-md border p-3">
                    <div><div className="font-medium">{pal.nickname || pal.type}</div><div className="text-xs text-muted-foreground">{pal.type}</div></div>
                    <div className="flex items-center gap-2"><Badge variant="secondary">Lv.{pal.level}</Badge>{pal.is_lucky ? <Sparkles className="h-4 w-4 text-amber-500" /> : null}</div>
                  </div>
                ))}</div>
              </section>
              <section>
                <h3 className="mb-3 text-sm font-semibold">背包与装备</h3>
                <div className="space-y-2">{allItems(player).filter((item) => item.StackCount > 0).slice(0, 100).map((item, index) => (
                  <div key={`${item.ItemId}-${item.SlotIndex}-${index}`} className="flex items-center justify-between border-b py-2 text-sm"><span className="font-mono text-xs">{item.ItemId}</span><Badge variant="outline">× {item.StackCount}</Badge></div>
                ))}</div>
              </section>
            </div>
          ) : null}
        </SheetBody>
      </SheetContent>
    </Sheet>
  )
}

function WorldWorkspace({ worldStatus, onRefreshSnapshot, refreshing }: { worldStatus: Awaited<ReturnType<typeof getWorldStatus>> | null; onRefreshSnapshot: () => void; refreshing: boolean }) {
  const guilds = useResource(getWorldGuilds, [worldStatus?.snapshot?.id])
  const camps = (guilds.data?.data ?? []).flatMap((guild) => guild.base_camp ?? [])
  return (
    <div className="flex flex-col gap-5">
      <PageStatStrip>
        <PageStat label="世界快照" value={worldStatus?.snapshot?.backupId || '未准备'} note={worldStatus?.snapshot?.createdAt || '读取完成的自动备份'} />
        <PageStat label="索引服务" value={worldStatus?.indexReachable ? '可用' : '等待启动'} note="固定版本只读侧车" />
        <PageStat label="公会" value={guilds.data?.data.length ?? '-'} note="来自 Level.sav" />
        <PageStat label="基地" value={camps.length || '-'} note="公会基地坐标" />
      </PageStatStrip>
      <PageSurface title="快照与索引" description="只解析已经完成的压缩备份，不读取游戏正在写入的世界文件。" actions={<Button onClick={onRefreshSnapshot} disabled={refreshing}><HardDriveDownload data-icon="inline-start" />{refreshing ? '准备中' : '准备最新快照'}</Button>}>
        <div className="grid gap-3 md:grid-cols-3">
          <MiniStat label="备份" value={worldStatus?.snapshot?.backupId || '-'} />
          <MiniStat label="快照时间" value={worldStatus?.snapshot?.createdAt || '-'} />
          <MiniStat label="索引 ID" value={worldStatus?.snapshot?.id || '-'} />
        </div>
      </PageSurface>
      <PageSurface title="公会与基地" description="成员、会长、基地等级与位置。">
        {guilds.error ? <ErrorState message={guilds.error} onRetry={guilds.refresh} /> : guilds.loading && !guilds.data ? <InlineLoader /> : !(guilds.data?.data.length) ? <EmptyState title="暂无公会数据" description="准备世界快照并等待索引完成后显示。" /> : (
          <Table className="min-w-[780px]"><TableHeader><TableRow><TableHead>公会</TableHead><TableHead>等级</TableHead><TableHead>成员</TableHead><TableHead>基地</TableHead><TableHead>会长 UID</TableHead></TableRow></TableHeader><TableBody>{guilds.data.data.map((guild) => <TableRow key={guild.admin_player_uid || guild.name}><TableCell className="font-medium">{guild.name || '未命名公会'}</TableCell><TableCell>{guild.base_camp_level}</TableCell><TableCell>{guild.players?.length ?? 0}</TableCell><TableCell>{guild.base_camp?.length ?? 0}</TableCell><TableCell className="font-mono text-xs">{guild.admin_player_uid || '-'}</TableCell></TableRow>)}</TableBody></Table>
        )}
      </PageSurface>
    </div>
  )
}

function EditorWorkspace() {
  const editor = useResource(getEditorStatus, [])
  const [action, setAction] = useState('player.stats')
  const [targetPlayer, setTargetPlayer] = useState('')
  const [changes, setChanges] = useState('{\n  "level": 50\n}')
  const [preview, setPreview] = useState<EditorPreview | null>(null)
  const [submitting, setSubmitting] = useState(false)
	const [sessionBusy, setSessionBusy] = useState(false)
  const { showToast } = useGlobalToast()

  const createPreview = async () => {
    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(changes) as Record<string, unknown>
    } catch {
      showToast('error', '变更内容必须是合法 JSON')
      return
    }
    setSubmitting(true)
    try {
      const result = await createEditorPreview({ action, targetPlayer, changes: parsed })
      setPreview(result)
      showToast('success', '维护编辑预览已创建')
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '预览创建失败')
    } finally {
      setSubmitting(false)
    }
  }

  const manageSession = async (sessionAction: 'start' | 'open' | 'stop') => {
    const editorTab = sessionAction === 'stop' ? null : window.open('about:blank', '_blank')
    setSessionBusy(true)
    try {
      const result = await runEditorSession(sessionAction)
      showToast('success', result.message)
      if (result.url) {
        if (editorTab) {
          editorTab.opener = null
          editorTab.location.href = result.url
        } else {
          window.location.assign(result.url)
        }
      } else {
        editorTab?.close()
      }
      editor.refresh()
    } catch (error) {
      editorTab?.close()
      showToast('error', error instanceof Error ? error.message : '维护会话操作失败')
    } finally {
      setSessionBusy(false)
    }
  }

  const canStartSession = Boolean(editor.data?.installed && editor.data.safety.canEditSnapshot)

  return (
    <div className="grid gap-5 xl:grid-cols-[minmax(0,0.9fr)_minmax(360px,1.1fr)]">
      <PageSurface title="编辑草稿" description="玩家、背包、帕鲁、公会和存档修复统一走预览。">
        <div className="space-y-4">
          <div className="space-y-2"><Label htmlFor="editor-action">动作</Label><select id="editor-action" value={action} onChange={(event) => setAction(event.target.value)} className="h-9 w-full rounded-md border bg-background px-3 text-sm"><option value="player.stats">玩家属性</option><option value="player.inventory">玩家背包</option><option value="player.map">地图与传送点</option><option value="pal.edit">帕鲁编辑</option><option value="guild.edit">公会编辑</option><option value="player.transfer">玩家迁移</option><option value="save.repair">存档修复</option></select></div>
          <div className="space-y-2"><Label htmlFor="target-player">目标 Player UID</Label><Input id="target-player" value={targetPlayer} onChange={(event) => setTargetPlayer(event.target.value)} placeholder="按动作需要填写" /></div>
          <div className="space-y-2"><Label htmlFor="editor-changes">变更 JSON</Label><Textarea id="editor-changes" value={changes} onChange={(event) => setChanges(event.target.value)} className="min-h-48 font-mono text-xs" /></div>
          <Button onClick={createPreview} disabled={submitting}><Wrench data-icon="inline-start" />{submitting ? '生成中' : '生成安全预览'}</Button>
        </div>
      </PageSurface>
      <PageSurface title="应用门禁" description="预览不写游戏存档；写回需要满足全部维护条件。">
        {editor.error ? <ErrorState message={editor.error} onRetry={editor.refresh} /> : editor.loading && !editor.data ? <InlineLoader /> : (
          <div className="space-y-5">
            <div className="grid gap-3 sm:grid-cols-3"><MiniStat label="编辑器" value={editor.data?.installed ? '已安装' : '等待安装'} /><MiniStat label="维护会话" value={editor.data?.reachable ? '已启动' : '未启动'} /><MiniStat label="生产写回" value={editor.data?.applyEnabled ? '已启用' : '关闭'} /></div>
            <div className="flex flex-wrap gap-2">
              {editor.data?.reachable ? <>
                <Button onClick={() => manageSession('open')} disabled={sessionBusy}><ExternalLink data-icon="inline-start" />打开编辑器</Button>
                <Button variant="outline" onClick={() => manageSession('stop')} disabled={sessionBusy}><PowerOff data-icon="inline-start" />停止会话</Button>
              </> : <Button onClick={() => manageSession('start')} disabled={sessionBusy || !canStartSession}><Power data-icon="inline-start" />启动维护会话</Button>}
            </div>
            <div className="space-y-2 text-sm">
              <SafetyRow ok={Boolean(editor.data?.safety.snapshotAvailable)} label="静态世界快照已准备" />
              <SafetyRow ok={!editor.data?.safety.gameRunning} label="游戏服务器已停止" />
              <SafetyRow ok={(editor.data?.safety.playersOnline ?? 0) === 0} label="在线玩家为 0" />
              <SafetyRow ok={Boolean(editor.data?.safety.applyEnabled)} label="生产写回开关已启用" />
            </div>
            {preview ? (
              <div className="rounded-md border border-amber-500/40 bg-amber-500/5 p-4">
                <div className="flex items-center justify-between gap-3"><span className="font-medium">预览 {preview.id}</span><Badge variant={preview.canApplyNow ? 'default' : 'outline'}>{preview.canApplyNow ? '可应用' : '已锁定'}</Badge></div>
                <pre className="mt-3 max-h-56 overflow-auto whitespace-pre-wrap font-mono text-xs">{JSON.stringify(preview.changes, null, 2)}</pre>
                {preview.blockedReasons.length ? <ul className="mt-3 space-y-1 text-sm text-muted-foreground">{preview.blockedReasons.map((reason) => <li key={reason}>- {reason}</li>)}</ul> : null}
              </div>
            ) : <EmptyState title="尚未创建预览" description="先选择动作并填写变更，面板会显示风险和阻断条件。" icon={<Backpack className="h-5 w-5" />} />}
          </div>
        )}
      </PageSurface>
    </div>
  )
}

function MiniStat({ label, value }: { label: string; value: string | number }) {
  return <div className="rounded-md border bg-muted/20 p-3"><div className="text-xs text-muted-foreground">{label}</div><div className="mt-1 truncate font-medium">{value}</div></div>
}

function SafetyRow({ ok, label }: { ok: boolean; label: string }) {
  return <div className="flex items-center justify-between gap-3 border-b py-2"><span>{label}</span><Badge variant={ok ? 'default' : 'outline'}>{ok ? '通过' : '未通过'}</Badge></div>
}

function allItems(player: WorldPlayer) {
  if (!player.items) return []
  return Object.values(player.items).flatMap((items) => items ?? [])
}

function countItems(player: WorldPlayer) {
  return allItems(player).filter((item) => item.StackCount > 0).length
}
