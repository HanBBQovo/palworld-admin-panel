import { useMemo, useState } from 'react'
import { Megaphone, Play, RefreshCw, Terminal } from 'lucide-react'

import { announceMessage, getLogs, getRconCommands, runRconCommand, type OperationRisk } from '@/api/palworld'
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
  const [announcement, setAnnouncement] = useState('')
  const [announcementResult, setAnnouncementResult] = useState('')
  const [announcing, setAnnouncing] = useState(false)
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

  const sendAnnouncement = async () => {
    const message = announcement.trim()
    if (!message) return
    setAnnouncing(true)
    try {
      const result = await announceMessage(message)
      setAnnouncementResult(`${result.sentAt} · ${result.transport}`)
      setAnnouncement('')
      showToast('success', result.message)
      logs.refresh()
    } catch (error) {
      const message = error instanceof Error ? error.message : '广播发送失败'
      setAnnouncementResult(message)
      showToast('error', message)
      logs.refresh()
    } finally {
      setAnnouncing(false)
    }
  }

  return (
    <PageShell
      title="控制台"
      description="发送中文广播、执行受控 RCON 命令并查看审计日志。"
      width="7xl"
      actions={
        <Button type="button" variant="outline" onClick={logs.refresh}>
          <RefreshCw data-icon="inline-start" />
          刷新日志
        </Button>
      }
    >
      <div className="flex flex-col gap-6">
        <PageStatStrip>
          <PageStat label="命令目录" value={commands.data?.length ?? 0} note="含用途与风险等级" />
          <PageStat label="执行方式" value="后端代理" note="前端不直连 RCON" />
          <PageStat label="安全确认" value="已启用" note="Shutdown / Ban 等二次确认" />
          <PageStat label="广播通道" value="官方 REST" note="UTF-8 中文文本" />
        </PageStatStrip>

        <PageSurface title="广播消息" description="公告通过 Palworld REST API 发送，支持中文和英文。">
          <div className="flex flex-col gap-3">
            <Textarea
              value={announcement}
              onChange={(event) => setAnnouncement(event.target.value)}
              maxLength={500}
              placeholder="输入发送给全部在线玩家的消息"
              className="min-h-24"
            />
            <div className="flex flex-wrap items-center gap-3">
              <Button type="button" onClick={sendAnnouncement} disabled={announcing || !announcement.trim()}>
                <Megaphone data-icon="inline-start" />
                {announcing ? '发送中' : '发送广播'}
              </Button>
              <span className="text-xs text-muted-foreground">{Array.from(announcement).length} / 500</span>
              {announcementResult ? <span className="text-sm text-muted-foreground">{announcementResult}</span> : null}
            </div>
          </div>
        </PageSurface>

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
          <PageSurface title="命令目录" description="点击一项会填入命令，右侧会显示它的实际 RCON 文本。">
            <div className="grid gap-3 md:grid-cols-2">
              {(commands.data ?? []).map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="rounded-md border border-border/70 p-4 text-left transition-colors hover:bg-muted/60"
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
                <div className="rounded-md border border-border/70 bg-muted/50 p-3">
                  <div className="mb-1 flex items-center gap-2">
                    <Badge variant={riskVariant[selectedDefinition.risk]}>{selectedDefinition.risk}</Badge>
                    <span className="text-sm font-medium">{selectedDefinition.label}</span>
                  </div>
                  <p className="text-sm text-muted-foreground">{selectedDefinition.description}</p>
                </div>
              ) : null}
              <Textarea value={command} onChange={(event) => setCommand(event.target.value)} className="min-h-28 font-mono" />
              <Button type="button" className="w-fit" onClick={() => execute()} disabled={running || !command.trim()}>
                <Play data-icon="inline-start" />
                提交给后端执行
              </Button>
              <div className="rounded-md border border-border/70 bg-muted/50 p-4">
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <Terminal className="size-4" />
                  最近一次执行结果
                </div>
                <pre className="max-h-72 min-h-36 overflow-y-auto whitespace-pre-wrap break-words rounded-md bg-background/60 p-3 font-mono text-xs text-muted-foreground">
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
              <div key={entry.id} className="rounded-md border border-border/70 p-3">
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
