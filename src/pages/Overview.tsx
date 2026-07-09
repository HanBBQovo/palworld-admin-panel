import { RefreshCw, ShieldCheck, Server } from 'lucide-react'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'

import { getLogs, getServerStatus, type LogEntry, type ServerHealth } from '@/api/palworld'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { type ChartConfig, ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart'
import { ErrorState } from '@/components/ui/error-state'
import { Progress } from '@/components/ui/progress'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { motion, staggerContainer, staggerItem } from '@/lib/motion'
import { useResource } from '@/lib/use-resource'

const chartConfig: ChartConfig = {
  usage: { label: '占用', color: 'hsl(var(--chart-1))' },
}

const healthLabel: Record<ServerHealth, string> = {
  healthy: '健康',
  starting: '启动中',
  warning: '需关注',
  offline: '离线',
}

const logTone: Record<LogEntry['level'], 'default' | 'secondary' | 'destructive' | 'outline'> = {
  info: 'secondary',
  warn: 'outline',
  error: 'destructive',
}

export default function Overview() {
  const statusResource = useResource(getServerStatus, [])
  const logResource = useResource(getLogs, [])
  const { data, loading, error, refresh } = statusResource

  const usageData = data
    ? [
        { name: 'CPU', usage: data.cpu },
        { name: '内存', usage: Math.round((data.memoryUsedGb / data.memoryLimitGb) * 100) },
        { name: '世界', usage: Math.round((data.worldSizeGb / Math.max(data.diskUsedGb, 1)) * 100) },
      ]
    : []

  const refreshAll = () => {
    statusResource.refresh()
    logResource.refresh()
  }

  return (
    <PageShell
      title="服务器总览"
      description="Palworld 专用服务器的运行状态、端口、安全边界和最近事件。"
      width="7xl"
      actions={
        <Button type="button" variant="outline" className="gap-2" onClick={refreshAll} disabled={loading}>
          <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
          刷新
        </Button>
      }
    >
      {error ? (
        <PageSurface>
          <ErrorState message={error} onRetry={refresh} />
        </PageSurface>
      ) : loading && !data ? (
        <div className="flex h-64 items-center justify-center">
          <InlineLoader />
        </div>
      ) : data ? (
        <motion.div variants={staggerContainer} initial="hidden" animate="visible" className="flex flex-col gap-6">
          <Alert>
            <ShieldCheck className="h-4 w-4" />
            <AlertTitle>管理口保持安全默认值</AlertTitle>
            <AlertDescription>
              游戏 UDP 8211/27015 可公网开放；RCON 25575 与 REST 8212 应仅绑定本机或内网，Web 面板后端负责代理管理动作。
            </AlertDescription>
          </Alert>

          <PageStatStrip>
            <motion.div variants={staggerItem}>
              <PageStat label="健康状态" value={healthLabel[data.health]} note={data.container} />
            </motion.div>
            <motion.div variants={staggerItem}>
              <PageStat label="在线玩家" value={`${data.playersOnline}/${data.playersMax}`} note={data.address} />
            </motion.div>
            <motion.div variants={staggerItem}>
              <PageStat label="服务端构建" value={data.version} note={data.image} />
            </motion.div>
            <motion.div variants={staggerItem}>
              <PageStat label="下次备份" value={data.nextBackupAt} note={`重启 ${data.nextRestartAt}`} />
            </motion.div>
          </PageStatStrip>

          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
            <PageSurface title="运行资源" description="来自当前容器的实时 Docker stats 与宿主机磁盘信息。">
              <div className="grid gap-4 md:grid-cols-3">
                <div className="rounded-xl border border-border/70 p-4">
                  <div className="mb-2 text-sm text-muted-foreground">CPU</div>
                  <div className="mb-3 text-2xl font-semibold">{data.cpu.toFixed(1)}%</div>
                  <Progress value={data.cpu} />
                </div>
                <div className="rounded-xl border border-border/70 p-4">
                  <div className="mb-2 text-sm text-muted-foreground">内存</div>
                  <div className="mb-3 text-2xl font-semibold">{data.memoryUsedGb.toFixed(1)}G / {data.memoryLimitGb.toFixed(1)}G</div>
                  <Progress value={(data.memoryUsedGb / data.memoryLimitGb) * 100} />
                </div>
                <div className="rounded-xl border border-border/70 p-4">
                  <div className="mb-2 text-sm text-muted-foreground">部署目录</div>
                  <div className="mb-3 text-2xl font-semibold">{data.diskUsedGb.toFixed(1)}G</div>
                  <Progress value={(data.diskUsedGb / data.diskTotalGb) * 100} />
                </div>
              </div>
              <div className="mt-6 h-56">
                <ChartContainer config={chartConfig} className="h-full w-full">
                  <BarChart data={usageData} margin={{ top: 8, right: 12, left: 0, bottom: 0 }}>
                    <CartesianGrid vertical={false} />
                    <XAxis dataKey="name" tickLine={false} axisLine={false} />
                    <YAxis tickLine={false} axisLine={false} tickFormatter={(value) => `${value}%`} />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Bar dataKey="usage" fill="var(--color-usage)" radius={[8, 8, 0, 0]} />
                  </BarChart>
                </ChartContainer>
              </div>
            </PageSurface>

            <PageSurface title="基础信息" description="当前真实部署路径和连接信息。">
              <div className="flex flex-col gap-3 text-sm">
                <InfoRow label="宿主机" value={data.host} />
                <InfoRow label="连接地址" value={data.address} />
                <InfoRow label={`启动时间（${data.timezone || '服务器时区'}）`} value={data.startedAt} />
                <InfoRow label="运行时长" value={data.uptime} />
                <InfoRow label="最后保存" value={data.lastSaveAt} />
                <InfoRow label="容器名" value={data.container} />
              </div>
            </PageSurface>
          </div>

          <div className="grid gap-4 xl:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
            <PageSurface title="端口策略" description="游戏走 UDP，管理口不裸露公网。">
              <Table className="min-w-[640px]">
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[120px]">端口</TableHead>
                    <TableHead className="w-[120px]">协议</TableHead>
                    <TableHead className="w-[120px]">暴露</TableHead>
                    <TableHead>用途</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.ports.map((port) => (
                    <TableRow key={`${port.port}-${port.protocol}`}>
                      <TableCell className="whitespace-nowrap font-mono">{port.port}</TableCell>
                      <TableCell className="whitespace-nowrap">{port.protocol}</TableCell>
                      <TableCell className="whitespace-nowrap">
                        <Badge variant={port.exposure === 'public' ? 'default' : 'secondary'}>
                          {port.exposure === 'public' ? '公网' : '本机'}
                        </Badge>
                      </TableCell>
                      <TableCell>{port.purpose}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </PageSurface>

            <PageSurface title="最近事件" description="启动、更新、备份和 RCON 输出。">
              <div className="flex flex-col gap-3">
                {(logResource.data ?? []).slice(0, 5).map((entry) => (
                  <div key={entry.id} className="flex gap-3 rounded-xl border border-border/70 p-3">
                    <Server className="mt-0.5 h-4 w-4 text-primary" />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <Badge variant={logTone[entry.level]}>{entry.level}</Badge>
                        <span className="text-xs text-muted-foreground">{entry.timestamp}</span>
                        <span className="text-xs text-muted-foreground">/{entry.source}</span>
                      </div>
                      <p className="mt-1 break-words text-sm">{entry.message}</p>
                    </div>
                  </div>
                ))}
              </div>
            </PageSurface>
          </div>
        </motion.div>
      ) : null}
    </PageShell>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg bg-muted/50 px-3 py-2">
      <span className="text-muted-foreground">{label}</span>
      <span className="truncate font-medium">{value}</span>
    </div>
  )
}
