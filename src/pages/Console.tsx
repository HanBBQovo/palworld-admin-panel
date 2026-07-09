import { useMemo, useState } from 'react'
import { Play, RefreshCw, Terminal } from 'lucide-react'

import { getLogs, getRconCommands, runRconCommand, type OperationRisk } from '@/api/palworld'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useConfirm } from '@/components/ui/use-confirm'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

const DANGEROUS_COMMANDS = ['Shutdown', 'DoExit', 'BanPlayer']

const riskVariant: Record<OperationRisk, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  low: 'secondary',
  medium: 'outline',
  high: 'destructive',
}

export default function Console() {
  const commands = useResource(getRconCommands, [])
  const logs = useResource(getLogs, [])
  const [command, setCommand] = useState('ShowPlayers')
  const [output, setOutput] = useState('')
  const [running, setRunning] = useState(false)
  const confirm = useConfirm()
  const { showToast } = useGlobalToast()

  const selectedDefinition = useMemo(
    () => (commands.data ?? []).find((item) => item.command === command),
    [command, commands.data],
  )

  const execute = async (nextCommand = command) => {
    const trimmed = nextCommand.trim()
    if (!trimmed) return
    const dangerous = DANGEROUS_COMMANDS.some((prefix) => trimmed.toLowerCase().startsWith(prefix.toLowerCase()))
    if (dangerous) {
      const ok = await confirm({
        title: '执行高风险 RCON 命令',
        description: `确认执行 ${trimmed}？这类命令可能封禁玩家、关服或中断当前游戏。`,
        confirmText: '确认执行',
        variant: 'destructive',
      })
      if (!ok) return
    }

    setRunning(true)
    try {
      const result = await runRconCommand(trimmed)
      setOutput(`[${result.executedAt}] $ ${result.command}\n${result.output}`)
      showToast('success', 'RCON 命令已提交')
      logs.refresh()
    } catch (error) {
      setOutput(error instanceof Error ? error.message : '执行失败')
      showToast('error', 'RCON 执行失败')
      logs.refresh()
    } finally {
      setRunning(false)
    }
  }

  return (
    <PageShell
      title="控制台"
      description="把 Palworld RCON 命令包装成可理解的操作目录；生产环境由后端在本机或 Compose 网络内执行。"
      width="7xl"
      actions={
        <Button type="button" variant="outline" className="gap-2" onClick={logs.refresh}>
          <RefreshCw className="h-4 w-4" />
          刷新日志
        </Button>
      }
    >
      <div className="flex flex-col gap-6">
        <PageStatStrip>
          <PageStat label="命令目录" value={commands.data?.length ?? 0} note="含用途与风险等级" />
          <PageStat label="执行方式" value="后端代理" note="前端不直连 RCON" />
          <PageStat label="安全确认" value="已启用" note="Shutdown / Ban 等二次确认" />
          <PageStat label="管理端口" value="25575" note="建议仅本机或内网" />
        </PageStatStrip>

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
          <PageSurface title="命令目录" description="点击一项会填入命令，右侧会显示它的实际 RCON 文本。">
            <div className="grid gap-3 md:grid-cols-2">
              {(commands.data ?? []).map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="rounded-xl border border-border/70 p-4 text-left transition-colors hover:bg-muted/60"
                  onClick={() => setCommand(item.command)}
                >
                  <div className="mb-2 flex items-center justify-between gap-2">
                    <span className="font-medium">{item.label}</span>
                    <Badge variant={riskVariant[item.risk]}>{item.risk}</Badge>
                  </div>
                  <div className="mb-2 font-mono text-xs text-muted-foreground">{item.command}</div>
                  <p className="text-sm leading-6 text-muted-foreground">{item.description}</p>
                </button>
              ))}
            </div>
          </PageSurface>

          <PageSurface title="执行命令" description="可直接编辑命令；包含 <SteamID> 的命令需要替换后再执行。">
            <div className="flex flex-col gap-4">
              {selectedDefinition ? (
                <div className="rounded-xl border border-border/70 bg-muted/50 p-3">
                  <div className="mb-1 flex items-center gap-2">
                    <Badge variant={riskVariant[selectedDefinition.risk]}>{selectedDefinition.risk}</Badge>
                    <span className="text-sm font-medium">{selectedDefinition.label}</span>
                  </div>
                  <p className="text-sm text-muted-foreground">{selectedDefinition.description}</p>
                </div>
              ) : null}
              <Textarea value={command} onChange={(event) => setCommand(event.target.value)} className="min-h-28 font-mono" />
              <Button type="button" className="w-fit gap-2" onClick={() => execute()} disabled={running || !command.trim()}>
                <Play className="h-4 w-4" />
                提交给后端执行
              </Button>
              <div className="rounded-xl border border-border/70 bg-muted/50 p-4">
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <Terminal className="h-4 w-4" />
                  最近一次执行结果
                </div>
                <pre className="max-h-72 min-h-36 overflow-y-auto whitespace-pre-wrap break-words rounded-lg bg-background/60 p-3 font-mono text-xs text-muted-foreground">
                  {output || '执行后会在这里显示返回内容；失败时也会显示错误原因。'}
                </pre>
              </div>
            </div>
          </PageSurface>
        </div>

        <PageSurface title="服务日志" description="启动、备份、更新、RCON 事件聚合。">
          <div className="max-h-[560px] overflow-y-auto pr-2">
            <div className="grid gap-3 xl:grid-cols-2">
            {(logs.data ?? []).map((entry) => (
              <div key={entry.id} className="rounded-xl border border-border/70 p-3">
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <Badge variant={entry.level === 'error' ? 'destructive' : entry.level === 'warn' ? 'outline' : 'secondary'}>{entry.level}</Badge>
                  <span className="text-xs text-muted-foreground">{entry.timestamp}</span>
                  <span className="text-xs text-muted-foreground">/{entry.source}</span>
                </div>
                <div className="break-words font-mono text-xs">{entry.message}</div>
              </div>
            ))}
            </div>
          </div>
        </PageSurface>
      </div>
    </PageShell>
  )
}
