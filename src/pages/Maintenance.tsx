import { useEffect, useState, type ReactNode } from 'react'
import { Power, RefreshCw, RotateCcw, Save, ShieldAlert, UploadCloud } from 'lucide-react'

import { getServerStatus, runMaintenanceAction, saveMaintenancePolicy, type MaintenancePolicy } from '@/api/palworld'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
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
  const [policy, setPolicy] = useState<MaintenancePolicy | null>(null)
  const [savingPolicy, setSavingPolicy] = useState(false)
  const [runningAction, setRunningAction] = useState<string | null>(null)
  const confirm = useConfirm()
  const { showToast } = useGlobalToast()

  useEffect(() => {
    if (data?.maintenance) setPolicy(data.maintenance)
  }, [data?.maintenance])

  const updatePolicy = <K extends keyof MaintenancePolicy>(key: K, value: MaintenancePolicy[K]) => {
    setPolicy((current) => current ? { ...current, [key]: value } : current)
  }

  const savePolicy = async () => {
    if (!policy) return
    const ok = await confirm({
      title: '保存自动维护策略',
      description: '会写入 .env；上游定时任务通常需要重启游戏容器后完全按新配置运行。确认保存？',
      confirmText: '保存策略',
    })
    if (!ok) return
    setSavingPolicy(true)
    try {
      const next = await saveMaintenancePolicy(policy)
      setPolicy(next)
      showToast('success', '自动维护策略已保存')
      refresh()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '保存失败')
    } finally {
      setSavingPolicy(false)
    }
  }

  const execute = async (action: ActionItem) => {
    const ok = await confirm({
      title: action.title,
      description: `${action.description} 确认继续？`,
      confirmText: action.title,
      variant: action.risk === 'high' ? 'destructive' : 'default',
    })
    if (!ok) return
    setRunningAction(action.key)
    try {
      const result = await runMaintenanceAction(action.key)
      showToast(result.ok ? 'success' : 'error', result.message)
      refresh()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '维护动作执行失败')
    } finally {
      setRunningAction(null)
    }
  }

  return (
    <PageShell
      title="维护更新"
      description="执行保存、重启、更新与关服，并管理自动维护窗口。"
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
          <PageStat label="自动更新" value={data?.maintenance.autoUpdate ? data.maintenance.autoUpdateCron : '关闭'} note="AUTO_UPDATE_CRON_EXPRESSION" />
          <PageStat label="自动重启" value={data?.maintenance.autoReboot ? data.maintenance.autoRebootCron : '关闭'} note="AUTO_REBOOT_CRON_EXPRESSION" />
          <PageStat label="自动备份" value={data?.maintenance.backupEnabled ? data.maintenance.backupCron : '关闭'} note={`保留 ${data?.maintenance.backupRetention ?? 0} 份`} />
        </PageStatStrip>

        {policy ? (
          <PageSurface
            title="自动维护策略"
            description="这里直接配置更新、重启、备份的开关和 Cron 时间；保存后写入同一个 .env。"
            actions={
              <Button type="button" className="gap-2" onClick={savePolicy} disabled={savingPolicy}>
                <Save className="h-4 w-4" />
                保存策略
              </Button>
            }
          >
            <div className="grid gap-4 xl:grid-cols-2">
              <PolicyRow
                title="容器启动时检查更新"
                description="控制 UPDATE_ON_BOOT。开启后重启游戏容器时会检查服务端更新。"
                checked={policy.updateOnBoot}
                onCheckedChange={(value) => updatePolicy('updateOnBoot', value)}
              />
              <PolicyRow
                title="自动更新"
                description="控制 AUTO_UPDATE_ENABLED。到点执行上游容器的自动更新逻辑。"
                checked={policy.autoUpdate}
                onCheckedChange={(value) => updatePolicy('autoUpdate', value)}
              >
                <Input value={policy.autoUpdateCron} onChange={(event) => updatePolicy('autoUpdateCron', event.target.value)} placeholder="0 4 * * *" />
              </PolicyRow>
              <PolicyRow
                title="自动重启"
                description="控制 AUTO_REBOOT_ENABLED。用于每天固定窗口重启游戏容器。"
                checked={policy.autoReboot}
                onCheckedChange={(value) => updatePolicy('autoReboot', value)}
              >
                <Input value={policy.autoRebootCron} onChange={(event) => updatePolicy('autoRebootCron', event.target.value)} placeholder="0 5 * * *" />
              </PolicyRow>
              <PolicyRow
                title="自动备份"
                description="控制 BACKUP_ENABLED。备份时间与保留份数都可以调整。"
                checked={policy.backupEnabled}
                onCheckedChange={(value) => updatePolicy('backupEnabled', value)}
              >
                <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_140px]">
                  <Input value={policy.backupCron} onChange={(event) => updatePolicy('backupCron', event.target.value)} placeholder="0 * * * *" />
                  <Input
                    type="number"
                    min={1}
                    value={policy.backupRetention}
                    onChange={(event) => updatePolicy('backupRetention', Math.max(1, Number(event.target.value) || 1))}
                  />
                </div>
              </PolicyRow>
            </div>
          </PageSurface>
        ) : null}

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
                    disabled={runningAction !== null}
                  >
                    {runningAction === action.key ? '执行中' : '执行'}
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

function PolicyRow({
  title,
  description,
  checked,
  onCheckedChange,
  children,
}: {
  title: string
  description: string
  checked: boolean
  onCheckedChange: (value: boolean) => void
  children?: ReactNode
}) {
  return (
    <div className="rounded-xl border border-border/70 p-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="text-sm font-medium">{title}</div>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
        </div>
        <Switch checked={checked} onCheckedChange={onCheckedChange} />
      </div>
      {children ? <div className="mt-4">{children}</div> : null}
    </div>
  )
}
