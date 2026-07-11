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
    const target = player.steamId
    if (!player.manageable || !target || target === '-') {
      showToast('error', '游戏当前未返回完整 Steam ID，不能安全执行玩家操作')
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
      description="来自服务端连接日志的实时名单，必要时回退 RCON。"
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
          <PageStat label="在线玩家" value={players.data?.length ?? 0} note="实时连接事件" />
          <PageStat label="服务器容量" value={status.data?.playersMax ?? '-'} note="当前配置上限" />
          <PageStat label="玩家数据源" value="连接日志 + RCON" note="优先使用加入/离开事件，避免名单被截断" />
          <PageStat label="可用操作" value="Kick / Ban" note="执行结果写入审计日志" />
        </PageStatStrip>

        <Alert>
          <ShieldAlert />
          <AlertTitle>玩家操作会立即生效</AlertTitle>
          <AlertDescription>
            踢出和封禁均经过二次确认。若当前 Palworld 版本通过 RCON 截断 Steam ID，面板会禁用操作，避免误踢或误封。
          </AlertDescription>
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
                          disabled={!player.manageable || runningPlayerId === player.id}
                          title={player.manageable ? '踢出该玩家' : '游戏未返回完整 Steam ID'}
                          onClick={() => runPlayerAction(player, 'kick')}
                        >
                          <UserX data-icon="inline-start" />
                          踢出
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          disabled={!player.manageable || runningPlayerId === player.id}
                          title={player.manageable ? '封禁该玩家' : '游戏未返回完整 Steam ID'}
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
