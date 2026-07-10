import { useState } from 'react'
import { Clock3, Copy, Database, HardDrive, RefreshCw, Save, Server, Users } from 'lucide-react'

import { getLogs, getServerStatus, runMaintenanceAction, type LogEntry, type ServerHealth } from '@/api/palworld'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ui/error-state'
import { Progress } from '@/components/ui/progress'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

const healthLabel: Record<ServerHealth, string> = {
  healthy: '运行正常',
  starting: '正在启动',
  warning: '需要检查',
  offline: '服务器离线',
}

const healthVariant: Record<ServerHealth, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  healthy: 'default',
  starting: 'secondary',
  warning: 'outline',
  offline: 'destructive',
}

const logTone: Record<LogEntry['level'], 'default' | 'secondary' | 'destructive' | 'outline'> = {
  info: 'secondary',
  warn: 'outline',
  error: 'destructive',
}

export default function Overview() {
  const status = useResource(getServerStatus, [])
  const logs = useResource(getLogs, [])
  const [saving, setSaving] = useState(false)
  const { showToast } = useGlobalToast()
  const { data, loading, error } = status

  const refreshAll = () => {
    status.refresh()
    logs.refresh()
  }

  const copyAddress = async () => {
    if (!data || data.address === '未配置连接域名') return
    await navigator.clipboard.writeText(data.address)
    showToast('success', '连接地址已复制')
  }

  const saveWorld = async () => {
    setSaving(true)
    try {
      const result = await runMaintenanceAction('rcon:save')
      showToast(result.ok ? 'success' : 'error', result.message)
      refreshAll()
    } catch (actionError) {
      showToast('error', actionError instanceof Error ? actionError.message : '保存世界失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <PageShell title="服务器状态台" description="连接、玩家、版本、存档与维护状态的实时视图。" width="7xl">
      {error ? (
        <PageSurface>
          <ErrorState message={error} onRetry={refreshAll} />
        </PageSurface>
      ) : loading && !data ? (
        <div className="flex h-64 items-center justify-center"><InlineLoader /></div>
      ) : data ? (
        <div className="flex flex-col gap-5">
          <PageSurface>
            <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
              <div className="min-w-0">
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <Badge variant={healthVariant[data.health]}>{healthLabel[data.health]}</Badge>
                  <span className="text-sm text-muted-foreground">{data.host}</span>
                </div>
                <h2 className="truncate text-2xl font-semibold">{data.name}</h2>
                <button
                  type="button"
                  className="mt-2 flex max-w-full items-center gap-2 font-mono text-sm text-muted-foreground hover:text-foreground"
                  onClick={copyAddress}
                  disabled={data.address === '未配置连接域名'}
                >
                  <span className="truncate">{data.address}</span>
                  {data.address !== '未配置连接域名' ? <Copy /> : null}
                </button>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="outline" onClick={refreshAll} disabled={loading}>
                  <RefreshCw data-icon="inline-start" className={loading ? 'animate-spin' : undefined} />
                  刷新状态
                </Button>
                <Button type="button" onClick={saveWorld} disabled={saving || data.health === 'offline'}>
                  <Save data-icon="inline-start" />
                  {saving ? '保存中' : '立即保存世界'}
                </Button>
              </div>
            </div>
          </PageSurface>

          {data.address === '未配置连接域名' ? (
            <Alert>
              <Server />
              <AlertTitle>玩家连接地址尚未配置</AlertTitle>
              <AlertDescription>在“服务器配置 → 网络与安全”中填写域名或公网 IP，状态台才会显示可复制的加入地址。</AlertDescription>
            </Alert>
          ) : null}

          <PageStatStrip>
            <PageStat label="在线玩家" value={`${data.playersOnline} / ${data.playersMax}`} note="来自 RCON ShowPlayers" />
            <PageStat label="游戏版本" value={data.gameVersion || data.version} note={data.versionSource === 'rcon' ? '来自 RCON Info' : '版本来源不可用'} />
            <PageStat label="运行时长" value={data.uptime} note={`启动于 ${data.startedAt}`} />
            <PageStat label="最后存档" value={data.lastSaveAt} note={`时区 ${data.timezone}`} />
          </PageStatStrip>

          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.3fr)_minmax(320px,0.7fr)]">
            <PageSurface title="运行资源" description="容器 CPU、内存和宿主机文件系统占用。">
              <div className="grid gap-4 md:grid-cols-3">
                <ResourceMeter icon={Server} label="CPU" value={data.cpu} detail={`${data.cpu.toFixed(1)}%`} />
                <ResourceMeter
                  icon={Database}
                  label="内存"
                  value={data.memoryLimitGb ? (data.memoryUsedGb / data.memoryLimitGb) * 100 : 0}
                  detail={`${data.memoryUsedGb.toFixed(1)} / ${data.memoryLimitGb.toFixed(1)} GB`}
                />
                <ResourceMeter
                  icon={HardDrive}
                  label="磁盘"
                  value={data.diskTotalGb ? (data.diskUsedGb / data.diskTotalGb) * 100 : 0}
                  detail={`${data.diskUsedGb.toFixed(1)} / ${data.diskTotalGb.toFixed(1)} GB`}
                />
              </div>
              <div className="mt-5 grid gap-3 border-t pt-5 sm:grid-cols-2">
                <InfoRow icon={Database} label="世界存档大小" value={`${data.worldSizeGb.toFixed(2)} GB`} />
                <InfoRow icon={Users} label="服务器容量" value={`${data.playersMax} 人`} />
              </div>
            </PageSurface>

            <PageSurface title="维护窗口" description="自动任务与当前部署标识。">
              <div className="flex flex-col gap-3">
                <InfoRow icon={Clock3} label="下次备份" value={data.nextBackupAt} />
                <InfoRow icon={Clock3} label="下次重启" value={data.nextRestartAt} />
                <InfoRow icon={Database} label="Steam Build ID" value={data.steamBuildId || 'unknown'} />
                <InfoRow icon={Server} label="容器" value={data.container} />
                <InfoRow icon={Server} label="镜像" value={data.image} />
              </div>
            </PageSurface>
          </div>

          <PageSurface title="最近事件" description="服务器、RCON、备份与更新日志。">
            {logs.error ? (
              <ErrorState message={logs.error} onRetry={logs.refresh} />
            ) : (
              <div className="grid gap-3 lg:grid-cols-2">
                {(logs.data ?? []).slice(0, 8).map((entry) => (
                  <div key={entry.id} className="border-l-2 border-border py-1 pl-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant={logTone[entry.level]}>{entry.level}</Badge>
                      <span className="text-xs text-muted-foreground">{entry.timestamp}</span>
                      <span className="text-xs text-muted-foreground">{entry.source}</span>
                    </div>
                    <p className="mt-1 break-words text-sm">{entry.message}</p>
                  </div>
                ))}
              </div>
            )}
          </PageSurface>
        </div>
      ) : null}
    </PageShell>
  )
}

function ResourceMeter({
  icon: Icon,
  label,
  value,
  detail,
}: {
  icon: typeof Server
  label: string
  value: number
  detail: string
}) {
  const safeValue = Math.max(0, Math.min(100, value))
  return (
    <div className="border p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <span className="flex items-center gap-2 text-sm text-muted-foreground"><Icon />{label}</span>
        <span className="font-mono text-sm">{detail}</span>
      </div>
      <Progress value={safeValue} />
    </div>
  )
}

function InfoRow({ icon: Icon, label, value }: { icon: typeof Server; label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b py-2 last:border-b-0">
      <span className="flex items-center gap-2 text-sm text-muted-foreground"><Icon />{label}</span>
      <span className="max-w-[65%] truncate text-right text-sm font-medium">{value}</span>
    </div>
  )
}
