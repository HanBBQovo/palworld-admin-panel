import { Ban, RefreshCw, ShieldAlert, UserX } from 'lucide-react'

import { getPlayers, type Player } from '@/api/palworld'
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
  const { data, loading, error, refresh } = useResource(getPlayers, [])
  const confirm = useConfirm()
  const { showToast } = useGlobalToast()
  const onlinePlayers = (data ?? []).filter((player) => player.status === 'online')

  const runPlayerAction = async (player: Player, action: 'kick' | 'ban') => {
    const confirmed = await confirm({
      title: action === 'kick' ? '踢出玩家' : '封禁玩家',
      description: `确认对 ${player.name} 执行 ${action === 'kick' ? 'KickPlayer' : 'BanPlayer'}？真实后端接入后会通过 RCON 执行。`,
      confirmText: action === 'kick' ? '踢出' : '封禁',
      variant: action === 'ban' ? 'destructive' : 'default',
    })
    if (!confirmed) return
    showToast('success', `${player.name} 的 ${action === 'kick' ? '踢出' : '封禁'} 指令已提交`)
  }

  return (
    <PageShell
      title="玩家管理"
      description="查看在线玩家，并为后端 RCON 接入预留踢出、封禁和传送操作。"
      width="7xl"
      actions={
        <Button type="button" variant="outline" className="gap-2" onClick={refresh} disabled={loading}>
          <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
          刷新玩家
        </Button>
      }
    >
      <div className="flex flex-col gap-6">
        <Alert>
          <ShieldAlert className="h-4 w-4" />
          <AlertTitle>危险玩家操作需要二次确认</AlertTitle>
          <AlertDescription>
            前端已统一使用确认弹窗；后端接入时仍需记录操作者、命令、目标 SteamID 和执行结果。
          </AlertDescription>
        </Alert>

        <PageStatStrip>
          <PageStat label="在线玩家" value={onlinePlayers.length} note="来自 ShowPlayers / REST players" />
          <PageStat label="最大人数" value="32" note="PLAYERS 环境变量" />
          <PageStat label="允许平台" value="4" note="Steam / Xbox / PS5 / Mac" />
          <PageStat label="管理动作" value="Kick / Ban" note="后端通过 RCON 执行" />
        </PageStatStrip>

        <PageSurface title="在线列表" description="当前 mock 使用部署快照，真实后端接入后可定时刷新。">
          {error ? (
            <ErrorState message={error} onRetry={refresh} />
          ) : loading && !data ? (
            <div className="flex h-48 items-center justify-center">
              <InlineLoader />
            </div>
          ) : !onlinePlayers.length ? (
            <EmptyState title="当前没有玩家在线" description="服务器已经可连接，等待玩家通过 域名:8211 或配置的公网地址加入。" />
          ) : (
            <Table className="min-w-[960px]">
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[240px]">玩家</TableHead>
                  <TableHead className="w-[110px]">平台</TableHead>
                  <TableHead className="w-[80px]">等级</TableHead>
                  <TableHead className="w-[150px]">公会</TableHead>
                  <TableHead>位置</TableHead>
                  <TableHead className="w-[100px]">延迟</TableHead>
                  <TableHead className="w-[190px] text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {onlinePlayers.map((player) => (
                  <TableRow key={player.id}>
                    <TableCell className="min-w-[240px]">
                      <div className="font-medium">{player.name}</div>
                      <div className="font-mono text-xs text-muted-foreground">{player.steamId}</div>
                    </TableCell>
                    <TableCell className="whitespace-nowrap"><Badge variant="secondary">{player.platform}</Badge></TableCell>
                    <TableCell className="whitespace-nowrap">{player.level}</TableCell>
                    <TableCell>{player.guild}</TableCell>
                    <TableCell>{player.location}</TableCell>
                    <TableCell className="whitespace-nowrap">{player.ping} ms</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2 whitespace-nowrap">
                        <Button type="button" variant="outline" size="sm" className="gap-2" onClick={() => runPlayerAction(player, 'kick')}>
                          <UserX className="h-4 w-4" />
                          踢出
                        </Button>
                        <Button type="button" variant="destructive" size="sm" className="gap-2" onClick={() => runPlayerAction(player, 'ban')}>
                          <Ban className="h-4 w-4" />
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
