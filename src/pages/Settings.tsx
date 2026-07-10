import { useEffect, useState } from 'react'
import { Save, ShieldAlert } from 'lucide-react'

import { getSettings, saveSettings, type ServerSettings } from '@/api/palworld'
import { FormField, FormSection } from '@/components/layout/FormScaffold'
import { PageSurface } from '@/components/layout/PageScaffold'
import { TabbedSettingsPage } from '@/components/layout/TabbedSettingsPage'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { MultiSelect } from '@/components/ui/multi-select'
import { Switch } from '@/components/ui/switch'
import { useResource } from '@/lib/use-resource'

type Tab = 'basic' | 'gameplay' | 'automation' | 'security'

const TABS: Array<{ key: Tab; label: string }> = [
  { key: 'basic', label: '基础配置' },
  { key: 'gameplay', label: '游戏参数' },
  { key: 'automation', label: '自动维护' },
  { key: 'security', label: '网络安全' },
]

const DEATH_PENALTY_OPTIONS = [
  { value: 'None', label: '无惩罚', description: '死亡不掉落物品' },
  { value: 'Item', label: '掉落物品', description: '只掉落背包物品' },
  { value: 'ItemAndEquipment', label: '物品和装备', description: '掉落物品与装备' },
  { value: 'All', label: '全部掉落', description: '当前服务器配置' },
]

const PLATFORM_OPTIONS = [
  { value: 'Steam', label: 'Steam' },
  { value: 'Xbox', label: 'Xbox' },
  { value: 'PS5', label: 'PS5' },
  { value: 'Mac', label: 'Mac' },
]

const numberValue = (value: string, fallback: number) => {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

export default function Settings() {
  const [activeTab, setActiveTab] = useState<Tab>('basic')
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [settings, setSettings] = useState<ServerSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const resource = useResource(getSettings, [])

  useEffect(() => {
    if (resource.data) setSettings(resource.data)
  }, [resource.data])

  const update = <K extends keyof ServerSettings>(key: K, value: ServerSettings[K]) => {
    setSettings((current) => current ? { ...current, [key]: value } : current)
  }

  const save = async () => {
    if (!settings) return
    setSaving(true)
    setMessage(null)
    try {
      const next = await saveSettings(settings)
      setSettings(next)
      setMessage({ type: 'success', text: '配置已经写入服务器；密码和游戏参数需要重启 Palworld 后生效。' })
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  if (!settings) {
    return (
      <TabbedSettingsPage
        title="参数设置"
        description="正在加载 Palworld 服务器配置。"
        tabs={TABS}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        indicatorId="palworld-settings-tabs"
      >
        <PageSurface title="加载中">读取配置中...</PageSurface>
      </TabbedSettingsPage>
    )
  }

  return (
    <TabbedSettingsPage
      title="服务器配置"
      description="编辑服务器基础信息、游戏倍率、自动维护和安全边界。"
      tabs={TABS}
      activeTab={activeTab}
      onTabChange={setActiveTab}
      indicatorId="palworld-settings-tabs"
      message={message}
      headerActions={
        <Button type="button" onClick={save} disabled={saving}>
          <Save className="h-4 w-4" />
          {saving ? '正在保存...' : '保存配置'}
        </Button>
      }
      extraContent={
        <div className="flex flex-col gap-3">
          {message ? (
            <Alert variant={message.type === 'error' ? 'destructive' : 'default'}>
              <Save />
              <AlertTitle>{message.type === 'error' ? '保存失败' : '保存成功'}</AlertTitle>
              <AlertDescription>{message.text}</AlertDescription>
            </Alert>
          ) : null}
          <Alert>
            <ShieldAlert />
            <AlertTitle>保存与生效是两个步骤</AlertTitle>
            <AlertDescription>
              点击保存会立即写入服务器配置文件；服务器密码、管理员密码和游戏参数需要在“维护任务”中重启 Palworld 后生效。
            </AlertDescription>
          </Alert>
        </div>
      }
    >
      {activeTab === 'basic' ? (
        <PageSurface title="基础配置" description="服务器名称、密码、人数和社区列表。">
          <FormSection className="sm:max-w-2xl">
            <FormField label="服务器名称" htmlFor="server-name" required>
              <Input id="server-name" value={settings.serverName} onChange={(event) => update('serverName', event.target.value)} />
            </FormField>
            <FormField label="服务器描述" htmlFor="server-description">
              <Input id="server-description" value={settings.description} onChange={(event) => update('description', event.target.value)} />
            </FormField>
            <div className="grid gap-5 sm:grid-cols-2">
              <FormField label="最大玩家数" htmlFor="players" description="Palworld 通常建议 1-32。">
                <Input
                  id="players"
                  type="number"
                  min={1}
                  max={32}
                  value={settings.players}
                  onChange={(event) => update('players', numberValue(event.target.value, 32))}
                />
              </FormField>
              <FormField label="显示到社区列表" description="公开展示时务必保留服务器密码。">
                <Switch checked={settings.community} onCheckedChange={(value) => update('community', value)} />
              </FormField>
            </div>
            <div className="grid gap-5 sm:grid-cols-2">
              <FormField label="服务器密码" htmlFor="server-password">
                <Input id="server-password" type="text" autoComplete="off" spellCheck={false} value={settings.serverPassword} onChange={(event) => update('serverPassword', event.target.value)} />
              </FormField>
              <FormField label="管理员密码" htmlFor="admin-password">
                <Input id="admin-password" type="text" autoComplete="off" spellCheck={false} value={settings.adminPassword} onChange={(event) => update('adminPassword', event.target.value)} />
              </FormField>
            </div>
            <FormField label="允许平台" description="需要保留括号格式，后端写入时应转成 (Steam,Xbox,PS5,Mac)。">
              <MultiSelect
                options={PLATFORM_OPTIONS}
                value={settings.crossplayPlatforms}
                onValueChange={(value) => update('crossplayPlatforms', value)}
                searchPlaceholder="搜索平台..."
              />
            </FormField>
          </FormSection>
        </PageSurface>
      ) : null}

      {activeTab === 'gameplay' ? (
        <PageSurface title="游戏倍率" description="常用平衡参数，后端应写入 compose environment。">
          <FormSection className="sm:max-w-3xl">
            <div className="grid gap-5 sm:grid-cols-3">
              <NumberField label="经验倍率" value={settings.expRate} onChange={(value) => update('expRate', value)} />
              <NumberField label="捕获倍率" value={settings.captureRate} onChange={(value) => update('captureRate', value)} />
              <NumberField label="刷新倍率" value={settings.spawnRate} onChange={(value) => update('spawnRate', value)} />
              <NumberField label="采集掉落" value={settings.collectionDropRate} onChange={(value) => update('collectionDropRate', value)} />
              <NumberField label="敌人掉落" value={settings.enemyDropRate} onChange={(value) => update('enemyDropRate', value)} />
              <NumberField label="孵蛋小时" value={settings.eggHatchingHours} onChange={(value) => update('eggHatchingHours', value)} />
              <NumberField label="自动保存秒" value={settings.autoSaveSpan} onChange={(value) => update('autoSaveSpan', value)} />
              <NumberField label="据点工作上限" value={settings.baseCampWorkerMax} onChange={(value) => update('baseCampWorkerMax', value)} />
              <NumberField label="公会人数上限" value={settings.guildPlayerMax} onChange={(value) => update('guildPlayerMax', value)} />
            </div>
            <FormField label="死亡惩罚">
              <Combobox
                options={DEATH_PENALTY_OPTIONS}
                value={settings.deathPenalty}
                onValueChange={(value) => update('deathPenalty', value as ServerSettings['deathPenalty'])}
                searchPlaceholder="搜索死亡惩罚..."
              />
            </FormField>
            <NumberField label="公会据点数量" value={settings.baseCampMaxInGuild} onChange={(value) => update('baseCampMaxInGuild', value)} />
          </FormSection>
        </PageSurface>
      ) : null}

      {activeTab === 'automation' ? (
        <PageSurface title="自动化功能" description="玩家日志、自动暂停、通知与版本锁定；定时更新和备份在“维护任务”中配置。">
          <FormSection className="sm:max-w-2xl">
            <ToggleRow
              title="自动暂停"
              description="无人在线时暂停 PalServer。需要 REST_API_ENABLED=true 和 ENABLE_PLAYER_LOGGING=true。"
              checked={settings.autoPauseEnabled}
              onCheckedChange={(value) => update('autoPauseEnabled', value)}
            />
            <ToggleRow
              title="玩家进出日志"
              description="用于在线玩家日志与自动暂停。"
              checked={settings.playerLoggingEnabled}
              onCheckedChange={(value) => update('playerLoggingEnabled', value)}
            />
            <ToggleRow
              title="Discord Webhook"
              description="备份、更新、失败事件推送到 Discord。"
              checked={settings.discordWebhookEnabled}
              onCheckedChange={(value) => update('discordWebhookEnabled', value)}
            />
            <FormField label="锁定 Steam manifest" htmlFor="manifest" description="更新翻车时临时锁版本；正常情况下留空。">
              <Input id="manifest" value={settings.targetManifestId} onChange={(event) => update('targetManifestId', event.target.value)} />
            </FormField>
          </FormSection>
        </PageSurface>
      ) : null}

      {activeTab === 'security' ? (
        <PageSurface title="网络安全" description="管理接口只应由后端本机访问，Web 面板走 HTTPS 鉴权入口。">
          <FormSection className="sm:max-w-2xl">
            <div className="grid gap-5 sm:grid-cols-2">
              <FormField label="连接域名" htmlFor="public-domain" description="玩家连接用域名，不要带端口；总览会展示为 域名:端口。">
                <Input
                  id="public-domain"
                  placeholder="pal.example.com"
                  value={settings.publicDomain}
                  onChange={(event) => update('publicDomain', event.target.value)}
                />
              </FormField>
              <FormField label="Public IP" htmlFor="public-ip" description="没有域名时可临时填写 IP。">
                <Input id="public-ip" placeholder="${PUBLIC_IP}" value={settings.publicIp} onChange={(event) => update('publicIp', event.target.value)} />
              </FormField>
              <FormField label="Public Port" htmlFor="public-port" description="默认 8211。">
                <Input id="public-port" placeholder="8211" value={settings.publicPort} onChange={(event) => update('publicPort', event.target.value)} />
              </FormField>
            </div>
            <ToggleRow
              title="RCON"
              description="当前开启，但仅绑定 127.0.0.1:25575。"
              checked={settings.rconEnabled}
              onCheckedChange={(value) => update('rconEnabled', value)}
            />
            <ToggleRow
              title="REST API"
              description="当前关闭。如启用，也应只绑定 127.0.0.1:8212。"
              checked={settings.restApiEnabled}
              onCheckedChange={(value) => update('restApiEnabled', value)}
            />
            <label className="flex items-start gap-3 rounded-xl border border-border/70 p-4 text-sm">
              <Checkbox checked disabled />
              <span>
                <span className="block font-medium">禁止公网裸露管理口</span>
                <span className="block text-muted-foreground">25575/RCON 和 8212/REST 只能由面板后端访问。</span>
              </span>
            </label>
          </FormSection>
        </PageSurface>
      ) : null}
    </TabbedSettingsPage>
  )
}

function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (value: number) => void }) {
  return (
    <FormField label={label}>
      <Input type="number" step="0.1" value={value} onChange={(event) => onChange(numberValue(event.target.value, value))} />
    </FormField>
  )
}

function ToggleRow({
  title,
  description,
  checked,
  onCheckedChange,
}: {
  title: string
  description: string
  checked: boolean
  onCheckedChange: (value: boolean) => void
}) {
  return (
    <label className="flex items-center justify-between gap-4 rounded-xl border border-border/70 p-4">
      <span>
        <span className="block text-sm font-medium">{title}</span>
        <span className="block text-xs leading-5 text-muted-foreground">{description}</span>
      </span>
      <Switch checked={checked} onCheckedChange={onCheckedChange} />
    </label>
  )
}
