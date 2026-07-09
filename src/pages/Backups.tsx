import { Archive, Download, RefreshCw, RotateCcw, ShieldAlert } from 'lucide-react'

import { getBackups, runMaintenanceAction, type Backup } from '@/api/palworld'
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

const statusVariant: Record<Backup['status'], 'default' | 'secondary' | 'destructive' | 'outline'> = {
  ready: 'secondary',
  running: 'outline',
  failed: 'destructive',
}

export default function Backups() {
  const { data, loading, error, refresh } = useResource(getBackups, [])
  const confirm = useConfirm()
  const { showToast } = useGlobalToast()

  const runAction = async (action: string, label: string, destructive = false) => {
    const ok = await confirm({
      title: label,
      description: destructive
        ? '恢复备份会覆盖当前世界存档。执行前建议确认没有玩家在线，并额外保留一份当前 SaveGames。'
        : '确认提交该维护动作？真实后端接入后会在游戏服务器所在主机或同 Compose 网络内执行 docker/rcon 脚本。',
      confirmText: label,
      variant: destructive ? 'destructive' : 'default',
    })
    if (!ok) return
    const result = await runMaintenanceAction(action)
    showToast(result.ok ? 'success' : 'error', result.message)
    refresh()
  }

  const readyCount = (data ?? []).filter((item) => item.status === 'ready').length

  return (
    <PageShell
      title="备份恢复"
      description="管理 Palworld 存档备份、手动备份和恢复动作。当前服务器每小时自动备份，保留 72 份。"
      width="7xl"
      actions={
        <>
          <Button type="button" variant="outline" className="gap-2" onClick={refresh} disabled={loading}>
            <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
            刷新
          </Button>
          <Button type="button" className="gap-2" onClick={() => runAction('backup:create', '创建手动备份')}>
            <Archive className="h-4 w-4" />
            创建备份
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-6">
        <Alert>
          <ShieldAlert className="h-4 w-4" />
          <AlertTitle>恢复是高风险操作</AlertTitle>
          <AlertDescription>
            面板会二次确认恢复动作。后端接入时建议先执行 RCON Save，再复制当前 SaveGames 到带时间戳目录。
          </AlertDescription>
        </Alert>

        <PageStatStrip>
          <PageStat label="自动备份" value="每小时" note="BACKUP_CRON_EXPRESSION=0 * * * *" />
          <PageStat label="保留份数" value="72" note="BACKUP_RETENTION_AMOUNT_TO_KEEP" />
          <PageStat label="可恢复备份" value={readyCount} note="/data/palworld-server/data/backups" />
          <PageStat label="当前策略" value="已启用" note="启动时生成 crontab" />
        </PageStatStrip>

        <PageSurface title="备份列表" description="生产接入后从 /data/palworld-server/data/backups 读取。">
          {error ? (
            <ErrorState message={error} onRetry={refresh} />
          ) : loading && !data ? (
            <div className="flex h-48 items-center justify-center">
              <InlineLoader />
            </div>
          ) : !data?.length ? (
            <EmptyState title="暂无备份" description="可点击右上角创建手动备份。" />
          ) : (
            <Table className="min-w-[940px]">
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[300px]">备份</TableHead>
                  <TableHead className="w-[180px]">时间</TableHead>
                  <TableHead className="w-[110px]">大小</TableHead>
                  <TableHead className="w-[100px]">类型</TableHead>
                  <TableHead className="w-[110px]">状态</TableHead>
                  <TableHead className="w-[220px] text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.map((backup) => (
                  <TableRow key={backup.id}>
                    <TableCell className="min-w-[300px]">
                      <div className="font-medium">{backup.id}</div>
                      <div className="text-xs text-muted-foreground">{backup.note}</div>
                    </TableCell>
                    <TableCell className="whitespace-nowrap">{backup.createdAt}</TableCell>
                    <TableCell className="whitespace-nowrap">{backup.size}</TableCell>
                    <TableCell className="whitespace-nowrap">{backup.type === 'automatic' ? '自动' : '手动'}</TableCell>
                    <TableCell className="whitespace-nowrap"><Badge variant={statusVariant[backup.status]}>{backup.status}</Badge></TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2 whitespace-nowrap">
                        <Button type="button" variant="outline" size="sm" className="gap-2" disabled={backup.status !== 'ready'}>
                          <Download className="h-4 w-4" />
                          下载
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          className="gap-2"
                          disabled={backup.status !== 'ready'}
                          onClick={() => runAction(`backup:restore:${backup.id}`, `恢复 ${backup.id}`, true)}
                        >
                          <RotateCcw className="h-4 w-4" />
                          恢复
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
