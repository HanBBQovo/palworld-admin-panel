import { Power, RefreshCw, RotateCcw, Save, ShieldAlert, UploadCloud } from 'lucide-react'

import { getServerStatus, runMaintenanceAction } from '@/api/palworld'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useConfirm } from '@/components/ui/use-confirm'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

interface ActionItem {
  key: string
  title: string
  description: string
  icon: typeof RefreshCw
  risk: 'low' | 'medium' | 'high'
}

const ACTIONS: ActionItem[] = [
  { key: 'rcon:save', title: '保存世界', description: '通过 RCON Save 立即保存当前世界状态。', icon: Save, risk: 'low' },
  { key: 'server:restart', title: '重启容器', description: 'docker compose restart，会触发 UPDATE_ON_BOOT 检查。', icon: RotateCcw, risk: 'medium' },
  { key: 'server:update', title: '拉取更新', description: 'docker compose pull + 重建容器，用于更新镜像与服务端。', icon: UploadCloud, risk: 'medium' },
  { key: 'server:shutdown', title: '优雅关服', description: '通过 RCON Shutdown 给玩家预告后关闭服务。', icon: Power, risk: 'high' },
]

const riskVariant: Record<ActionItem['risk'], 'default' | 'secondary' | 'destructive' | 'outline'> = {
  low: 'secondary',
  medium: 'outline',
  high: 'destructive',
}

export default function Maintenance() {
  const { data, refresh } = useResource(getServerStatus, [])
  const confirm = useConfirm()
  const { showToast } = useGlobalToast()

  const execute = async (action: ActionItem) => {
    const ok = await confirm({
      title: action.title,
      description: `${action.description} 确认继续？`,
      confirmText: action.title,
      variant: action.risk === 'high' ? 'destructive' : 'default',
    })
    if (!ok) return
    const result = await runMaintenanceAction(action.key)
    showToast(result.ok ? 'success' : 'error', result.message)
    refresh()
  }

  return (
    <PageShell
      title="维护更新"
      description="集中执行保存、重启、更新、关服，以及查看自动维护策略。"
      width="7xl"
      actions={
        <Button type="button" variant="outline" className="gap-2" onClick={refresh}>
          <RefreshCw className="h-4 w-4" />
          刷新策略
        </Button>
      }
    >
      <div className="flex flex-col gap-6">
        <Alert>
          <ShieldAlert className="h-4 w-4" />
          <AlertTitle>维护动作会影响在线玩家</AlertTitle>
          <AlertDescription>
            重启、更新、关服等动作必须由后端加审计日志，并建议先广播通知玩家。
          </AlertDescription>
        </Alert>

        <PageStatStrip>
          <PageStat label="启动更新" value={data?.maintenance.updateOnBoot ? '开启' : '关闭'} note="UPDATE_ON_BOOT" />
          <PageStat label="自动更新" value={data?.maintenance.autoUpdate ? '04:00' : '关闭'} note={data?.maintenance.autoUpdateCron} />
          <PageStat label="自动重启" value={data?.maintenance.autoReboot ? '05:00' : '关闭'} note={data?.maintenance.autoRebootCron} />
          <PageStat label="自动备份" value={data?.maintenance.backupEnabled ? '每小时' : '关闭'} note={`保留 ${data?.maintenance.backupRetention ?? 0} 份`} />
        </PageStatStrip>

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {ACTIONS.map((action) => {
            const Icon = action.icon
            return (
              <PageSurface key={action.key} title={action.title} description={action.description}>
                <div className="flex flex-col gap-4">
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                      <Icon className="h-5 w-5" />
                    </div>
                    <Badge variant={riskVariant[action.risk]}>{action.risk}</Badge>
                  </div>
                  <Button
                    type="button"
                    variant={action.risk === 'high' ? 'destructive' : 'default'}
                    className="w-full"
                    onClick={() => execute(action)}
                  >
                    执行
                  </Button>
                </div>
              </PageSurface>
            )
          })}
        </div>
      </div>
    </PageShell>
  )
}
