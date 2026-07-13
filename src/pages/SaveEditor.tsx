import { ExternalLink, HardDriveDownload, LockKeyhole, Power, PowerOff, RefreshCw, ShieldCheck } from 'lucide-react'
import { useState } from 'react'

import { getEditorStatus, getWorldStatus, refreshWorldSnapshot, runEditorSession } from '@/api/palworld'
import { InlineLoader } from '@/components/PageLoader'
import { PageShell, PageStat, PageStatStrip, PageSurface } from '@/components/layout/PageScaffold'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ErrorState } from '@/components/ui/error-state'
import { useGlobalToast } from '@/components/ui/use-global-toast'
import { useResource } from '@/lib/use-resource'

const actionLabels: Record<string, string> = {
  'player.stats': '玩家属性',
  'player.inventory': '玩家背包',
  'player.map': '地图与传送点',
  'pal.edit': '帕鲁编辑',
  'guild.edit': '公会编辑',
  'player.transfer': '玩家迁移',
  'save.repair': '存档修复',
}

export default function SaveEditor() {
  const editor = useResource(getEditorStatus, [], { refreshIntervalMs: 10_000 })
  const world = useResource(getWorldStatus, [], { refreshIntervalMs: 60_000 })
  const [sessionBusy, setSessionBusy] = useState(false)
  const [refreshingSnapshot, setRefreshingSnapshot] = useState(false)
  const { showToast } = useGlobalToast()
  const canStart = Boolean(editor.data?.installed && editor.data.safety.canEditSnapshot)

  const manageSession = async (action: 'start' | 'open' | 'stop') => {
    const editorTab = action === 'stop' ? null : window.open('about:blank', '_blank')
    setSessionBusy(true)
    try {
      const result = await runEditorSession(action)
      showToast('success', result.message)
      if (result.url) {
        if (editorTab) {
          editorTab.opener = null
          editorTab.location.href = result.url
        } else {
          window.location.assign(result.url)
        }
      } else {
        editorTab?.close()
      }
      editor.refresh()
    } catch (error) {
      editorTab?.close()
      showToast('error', error instanceof Error ? error.message : '维护编辑器操作失败')
    } finally {
      setSessionBusy(false)
    }
  }

  const refreshSnapshot = async () => {
    setRefreshingSnapshot(true)
    try {
      const result = await refreshWorldSnapshot()
      showToast('success', result.message)
      world.refresh()
      editor.refresh()
    } catch (error) {
      showToast('error', error instanceof Error ? error.message : '快照刷新失败')
    } finally {
      setRefreshingSnapshot(false)
    }
  }

  return (
    <PageShell
      title="存档编辑"
      description="停服维护时启动图形编辑器，不再填写 JSON。"
      width="6xl"
      actions={
        <Button variant="outline" onClick={() => { editor.refresh(); world.refresh() }}>
          <RefreshCw data-icon="inline-start" />
          刷新状态
        </Button>
      }
    >
      <div className="flex flex-col gap-5">
        {editor.data?.safety.gameRunning ? (
          <Alert>
            <LockKeyhole />
            <AlertTitle>游戏运行中，存档编辑已锁定</AlertTitle>
            <AlertDescription>当前只能更新维护快照。停止游戏且在线人数为 0 后，启动按钮会自动可用。</AlertDescription>
          </Alert>
        ) : (
          <Alert>
            <ShieldCheck />
            <AlertTitle>维护窗口已打开</AlertTitle>
            <AlertDescription>确认快照时间后启动图形编辑器，完成修改后从编辑器导出存档。</AlertDescription>
          </Alert>
        )}

        <PageStatStrip>
          <PageStat label="编辑器" value={editor.data?.installed ? '已安装' : '未安装'} note="Palworld Save Pal v0.17.4" />
          <PageStat label="维护会话" value={editor.data?.reachable ? '运行中' : '未启动'} note="仅回环地址访问" />
          <PageStat label="维护快照" value={world.data?.snapshot.backupId || '-'} note={world.data?.snapshot.createdAt || '等待快照'} />
          <PageStat label="自动写回" value={editor.data?.applyEnabled ? '已启用' : '关闭'} note="修改不会直接覆盖在线世界" />
        </PageStatStrip>

        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.72fr)]">
          <PageSurface
            title="图形编辑器"
            description="加载维护快照后，在独立页面选择玩家、帕鲁、背包或公会进行修改。"
            actions={
              <Button variant="outline" onClick={refreshSnapshot} disabled={refreshingSnapshot}>
                <HardDriveDownload data-icon="inline-start" />
                {refreshingSnapshot ? '更新中' : '更新维护快照'}
              </Button>
            }
          >
            {editor.error ? (
              <ErrorState message={editor.error} onRetry={editor.refresh} />
            ) : editor.loading && !editor.data ? (
              <div className="flex h-48 items-center justify-center"><InlineLoader /></div>
            ) : (
              <div className="flex flex-col gap-5">
                <div className="flex flex-wrap gap-2">
                  {(editor.data?.supportedActions ?? []).map((action) => (
                    <Badge key={action} variant="secondary">{actionLabels[action] ?? action}</Badge>
                  ))}
                </div>

                <div className="flex flex-wrap gap-2">
                  {editor.data?.reachable ? (
                    <>
                      <Button onClick={() => manageSession('open')} disabled={sessionBusy}>
                        <ExternalLink data-icon="inline-start" />
                        打开图形编辑器
                      </Button>
                      <Button variant="outline" onClick={() => manageSession('stop')} disabled={sessionBusy}>
                        <PowerOff data-icon="inline-start" />
                        停止会话
                      </Button>
                    </>
                  ) : (
                    <Button onClick={() => manageSession('start')} disabled={sessionBusy || !canStart}>
                      <Power data-icon="inline-start" />
                      启动图形编辑器
                    </Button>
                  )}
                </div>
              </div>
            )}
          </PageSurface>

          <PageSurface title="安全门禁" description="全部条件通过后才能启动维护会话。">
            <div className="flex flex-col gap-1 text-sm">
              <SafetyRow ok={Boolean(editor.data?.installed)} label="编辑器镜像已安装" />
              <SafetyRow ok={Boolean(editor.data?.safety.snapshotAvailable)} label="维护快照已准备" />
              <SafetyRow ok={!editor.data?.safety.gameRunning} label="游戏服务器已停止" />
              <SafetyRow ok={(editor.data?.safety.playersOnline ?? 0) === 0} label="在线玩家为 0" />
            </div>
          </PageSurface>
        </div>
      </div>
    </PageShell>
  )
}

function SafetyRow({ ok, label }: { ok: boolean; label: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b py-3">
      <span>{label}</span>
      <Badge variant={ok ? 'default' : 'outline'}>{ok ? '通过' : '未通过'}</Badge>
    </div>
  )
}
