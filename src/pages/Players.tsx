import { useState } from 'react'
import { Ban, RefreshCw, ShieldAlert, UserX } from 'lucide-react'

import { getPlayers, getServerStatus, runRconCommand, type Player } from '@/api/palworld'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { EmptyState } from '@/components/ui/empty-state'
import { ErrorState } from '@/components/ui/error-state'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useConfirm } from '@/components/ui/use-confirm'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

export default function Players() {
  const players = useResource(getPlayers, [])
  const status = useResource(getServerStatus, [])
  const [runningPlayerId, setRunningPlayerId] = useState<string | null>(null)
  const confirm = useConfirm()
  const { showToast } = useGlobalToast()

  const refreshAll = () => {
    players.refresh()
    status.refresh()
  }

  const runPlayerAction = async (player: Player, action: 'kick' | 'ban') => {
    const target = player.steamId && player.steamId !== '-' ? player.steamId : player.playerUid
    if (!target || target === '-') {
      showToast('error', '该玩家没有可用的 RCON 标识')
      return
    }
    const command = `${action === 'kick' ? 'KickPlayer' : 'BanPlayer'} ${target}`
    const confirmed = await confirm({
      title: action === 'kick' ? '踢出玩家' : '封禁玩家',
      description: `将通过 RCON 执行：${command}`,
      confirmText: action === 'kick' ? '踢出' : '封禁',
      variant: action === 'ban' ? 'destructive' : 'default',
    })
    if (!confirmed) return

    setRunningPlayerId(player.id)
    try {
      const result = await runRconCommand(command)
      showToast('success', result.output || `${player.name} 操作完成`)
      refreshAll()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '玩家操作失败')
    } finally {
      setRunningPlayerId(null)
    }
  }

  return (
    <PageShell
      title="在线玩家"
      description="来自 RCON ShowPlayers 的实时名单与管理操作。"
      width="7xl"
      actions={
        <Button type="button" variant="outline" onClick={refreshAll} disabled={players.loading}>
          <RefreshCw data-icon="inline-start" className={players.loading ? 'animate-spin' : undefined} />
          刷新玩家
        </Button>
      }
    >
      <div className="flex flex-col gap-5">
        <PageStatStrip>
          <PageStat label="在线玩家" value={players.data?.length ?? 0} note="实时 RCON 查询" />
          <PageStat label="服务器容量" value={status.data?.playersMax ?? '-'} note="当前配置上限" />
          <PageStat label="玩家数据源" value="ShowPlayers" note="不展示 RCON 未提供的伪字段" />
          <PageStat label="可用操作" value="Kick / Ban" note="执行结果写入审计日志" />
        </PageStatStrip>

        <Alert>
          <ShieldAlert />
          <AlertTitle>玩家操作会立即生效</AlertTitle>
          <AlertDescription>踢出和封禁均经过二次确认，并由后端在本机通过 RCON 执行。</AlertDescription>
        </Alert>

        <PageSurface title="当前在线名单" description="Player UID 与 Steam ID 均直接来自 Palworld RCON。">
          {players.error ? (
            <ErrorState message={players.error} onRetry={players.refresh} />
          ) : players.loading && !players.data ? (
            <div className="flex h-48 items-center justify-center"><InlineLoader /></div>
          ) : !players.data?.length ? (
            <EmptyState title="当前没有玩家在线" description="服务器可用时，这里会显示通过 RCON 获取的实时玩家名单。" />
          ) : (
            <Table className="min-w-[760px]">
              <TableHeader>
                <TableRow>
                  <TableHead>玩家</TableHead>
                  <TableHead>平台</TableHead>
                  <TableHead>Player UID</TableHead>
                  <TableHead>Steam ID</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {players.data.map((player) => (
                  <TableRow key={player.id}>
                    <TableCell className="font-medium">{player.name}</TableCell>
                    <TableCell><Badge variant="secondary">{player.platform}</Badge></TableCell>
                    <TableCell className="font-mono text-xs">{player.playerUid}</TableCell>
                    <TableCell className="font-mono text-xs">{player.steamId}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={runningPlayerId === player.id}
                          onClick={() => runPlayerAction(player, 'kick')}
                        >
                          <UserX data-icon="inline-start" />
                          踢出
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          disabled={runningPlayerId === player.id}
                          onClick={() => runPlayerAction(player, 'ban')}
                        >
                          <Ban data-icon="inline-start" />
                          封禁
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </PageSurface>
      </div>
    </PageShell>
  )
}
