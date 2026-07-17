import { useState } from 'react'

import { DEFAULT_GAME_PARAMETERS, type GameParameterName } from '@/api/palworld'
import { ScrollableTabBar } from '@/components/layout/ScrollableTabBar'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

type ParameterGroup = 'world' | 'player' | 'pal' | 'building' | 'guild' | 'advanced'
type ParameterKind = 'boolean' | 'number' | 'text' | 'select'

interface ParameterOption {
  value: string
  label: string
  description?: string
}

interface ParameterDefinition {
  group: ParameterGroup
  label: string
  description: string
  kind: ParameterKind
  step?: number | 'any'
  options?: ParameterOption[]
}

interface GameParameterFieldsProps {
  value: Record<GameParameterName, string>
  onChange: (value: Record<GameParameterName, string>) => void
}

const PARAMETER_GROUPS: Array<{ key: ParameterGroup; label: string; description: string }> = [
  { key: 'world', label: '世界', description: '难度、昼夜、随机化和世界事件。' },
  { key: 'player', label: '玩家与战斗', description: '玩家生存、伤害、复活和语音规则。' },
  { key: 'pal', label: '帕鲁与生态', description: '帕鲁属性、养成、农场和终端流转。' },
  { key: 'building', label: '建造与掉落', description: '建筑耐久、采集刷新、掉落和据点限制。' },
  { key: 'guild', label: '公会与 PvP', description: '多人、公会治理和 PvP 行为。' },
  { key: 'advanced', label: '高级规则', description: '鉴权、封禁、日志和客户端兼容规则。' },
]

const DIFFICULTY_OPTIONS: ParameterOption[] = [
  { value: 'None', label: '自定义', description: '使用本页配置的各项参数' },
  { value: 'Normal', label: '普通' },
  { value: 'Difficult', label: '困难' },
]

const RANDOMIZER_OPTIONS: ParameterOption[] = [
  { value: 'None', label: '关闭', description: '使用正常帕鲁分布' },
  { value: 'Region', label: '按区域随机' },
  { value: 'All', label: '完全随机' },
]

const PARAMETER_DEFINITIONS = {
  DIFFICULTY: {
    group: 'world',
    label: '难度预设',
    description: '选择预设；自定义时使用本页的详细倍率。',
    kind: 'select',
    options: DIFFICULTY_OPTIONS,
  },
  RANDOMIZER_TYPE: {
    group: 'world',
    label: '帕鲁随机化模式',
    description: '控制野外帕鲁是否按区域或全局随机。',
    kind: 'select',
    options: RANDOMIZER_OPTIONS,
  },
  RANDOMIZER_SEED: {
    group: 'world',
    label: '随机化种子',
    description: '相同种子可复现相同的随机化结果；留空自动生成。',
    kind: 'text',
  },
  IS_RANDOMIZER_PAL_LEVEL_RANDOM: {
    group: 'world',
    label: '随机帕鲁等级',
    description: '随机化开启时，同时随机野生帕鲁等级。',
    kind: 'boolean',
  },
  DAYTIME_SPEEDRATE: {
    group: 'world',
    label: '白天流速倍率',
    description: '数值越大，白天经过得越快。',
    kind: 'number',
  },
  NIGHTTIME_SPEEDRATE: {
    group: 'world',
    label: '夜晚流速倍率',
    description: '数值越大，夜晚经过得越快。',
    kind: 'number',
  },
  PAL_DAMAGE_RATE_ATTACK: {
    group: 'pal',
    label: '帕鲁攻击伤害倍率',
    description: '调整帕鲁造成的伤害。',
    kind: 'number',
  },
  PAL_DAMAGE_RATE_DEFENSE: {
    group: 'pal',
    label: '帕鲁承伤倍率',
    description: '调整帕鲁受到的伤害。',
    kind: 'number',
  },
  PLAYER_DAMAGE_RATE_ATTACK: {
    group: 'player',
    label: '玩家攻击伤害倍率',
    description: '调整玩家造成的伤害。',
    kind: 'number',
  },
  PLAYER_DAMAGE_RATE_DEFENSE: {
    group: 'player',
    label: '玩家承伤倍率',
    description: '调整玩家受到的伤害。',
    kind: 'number',
  },
  PLAYER_STOMACH_DECREASE_RATE: {
    group: 'player',
    label: '玩家饱食消耗倍率',
    description: '数值越大，饱食度下降越快。',
    kind: 'number',
  },
  PLAYER_STAMINA_DECREASE_RATE: {
    group: 'player',
    label: '玩家耐力消耗倍率',
    description: '数值越大，耐力消耗越快。',
    kind: 'number',
  },
  PLAYER_AUTO_HP_REGEN_RATE: {
    group: 'player',
    label: '玩家生命恢复倍率',
    description: '调整玩家通常状态下的生命恢复速度。',
    kind: 'number',
  },
  PLAYER_AUTO_HP_REGEN_RATE_IN_SLEEP: {
    group: 'player',
    label: '玩家睡眠恢复倍率',
    description: '调整玩家睡眠时的生命恢复速度。',
    kind: 'number',
  },
  PAL_STOMACH_DECREASE_RATE: {
    group: 'pal',
    label: '帕鲁饱食消耗倍率',
    description: '数值越大，帕鲁饱食度下降越快。',
    kind: 'number',
  },
  PAL_STAMINA_DECREASE_RATE: {
    group: 'pal',
    label: '帕鲁耐力消耗倍率',
    description: '数值越大，帕鲁耐力消耗越快。',
    kind: 'number',
  },
  PAL_AUTO_HP_REGEN_RATE: {
    group: 'pal',
    label: '帕鲁生命恢复倍率',
    description: '调整帕鲁通常状态下的生命恢复速度。',
    kind: 'number',
  },
  PAL_AUTO_HP_REGEN_RATE_IN_SLEEP: {
    group: 'pal',
    label: '帕鲁睡眠恢复倍率',
    description: '调整帕鲁睡眠时的生命恢复速度。',
    kind: 'number',
  },
  BUILD_OBJECT_HP_RATE: {
    group: 'building',
    label: '建筑生命倍率',
    description: '调整建筑物的耐久上限。',
    kind: 'number',
  },
  BUILD_OBJECT_DAMAGE_RATE: {
    group: 'building',
    label: '建筑承伤倍率',
    description: '调整建筑物受到的直接伤害。',
    kind: 'number',
  },
  BUILD_OBJECT_DETERIORATION_DAMAGE_RATE: {
    group: 'building',
    label: '建筑腐朽倍率',
    description: '调整据点范围外建筑的自然损耗。',
    kind: 'number',
  },
  COLLECTION_OBJECT_HP_RATE: {
    group: 'building',
    label: '采集物耐久倍率',
    description: '调整矿石、树木等采集物的生命值。',
    kind: 'number',
  },
  COLLECTION_OBJECT_RESPAWN_SPEED_RATE: {
    group: 'building',
    label: '采集物刷新倍率',
    description: '调整自然资源重新生成的速度。',
    kind: 'number',
  },
  ENABLE_PLAYER_TO_PLAYER_DAMAGE: {
    group: 'guild',
    label: '玩家间伤害',
    description: '允许玩家直接对其他玩家造成伤害。',
    kind: 'boolean',
  },
  ENABLE_FRIENDLY_FIRE: {
    group: 'guild',
    label: '友军伤害',
    description: '允许对同公会玩家或友方目标造成伤害。',
    kind: 'boolean',
  },
  ENABLE_INVADER_ENEMY: {
    group: 'world',
    label: '据点袭击事件',
    description: '允许敌人发起据点入侵事件。',
    kind: 'boolean',
  },
  ACTIVE_UNKO: {
    group: 'advanced',
    label: '启用 UNKO 规则',
    description: '控制服务端的 UNKO 特殊规则。',
    kind: 'boolean',
  },
  ENABLE_AIM_ASSIST_PAD: {
    group: 'player',
    label: '手柄辅助瞄准',
    description: '允许手柄玩家使用辅助瞄准。',
    kind: 'boolean',
  },
  ENABLE_AIM_ASSIST_KEYBOARD: {
    group: 'player',
    label: '键鼠辅助瞄准',
    description: '允许键鼠玩家使用辅助瞄准。',
    kind: 'boolean',
  },
  DROP_ITEM_MAX_NUM: {
    group: 'building',
    label: '世界掉落物上限',
    description: '世界中可同时存在的普通掉落物数量。',
    kind: 'number',
    step: 1,
  },
  DROP_ITEM_MAX_NUM_UNKO: {
    group: 'building',
    label: 'UNKO 掉落物上限',
    description: '世界中可同时存在的 UNKO 掉落物数量。',
    kind: 'number',
    step: 1,
  },
  BASE_CAMP_MAX_NUM: {
    group: 'building',
    label: '全服据点总上限',
    description: '限制整个世界可创建的据点总数。',
    kind: 'number',
    step: 1,
  },
  DROP_ITEM_ALIVE_MAX_HOURS: {
    group: 'building',
    label: '掉落物保留小时',
    description: '掉落物在世界中保留的最长时间。',
    kind: 'number',
  },
  AUTO_RESET_GUILD_NO_ONLINE_PLAYERS: {
    group: 'guild',
    label: '自动清理离线公会',
    description: '长期无人上线时允许自动重置公会。',
    kind: 'boolean',
  },
  AUTO_RESET_GUILD_TIME_NO_ONLINE_PLAYERS: {
    group: 'guild',
    label: '离线公会清理小时',
    description: '公会无人在线达到该时长后执行重置。',
    kind: 'number',
  },
  WORK_SPEED_RATE: {
    group: 'player',
    label: '玩家工作速度倍率',
    description: '调整玩家手工作业和建造速度。',
    kind: 'number',
  },
  IS_MULTIPLAY: {
    group: 'guild',
    label: '多人游戏规则',
    description: '启用游戏内多人模式相关规则。',
    kind: 'boolean',
  },
  IS_PVP: {
    group: 'guild',
    label: 'PvP 模式',
    description: '启用服务器 PvP 规则。',
    kind: 'boolean',
  },
  HARDCORE: {
    group: 'player',
    label: '硬核模式',
    description: '启用硬核死亡规则。',
    kind: 'boolean',
  },
  CHARACTER_RECREATE_IN_HARDCORE: {
    group: 'player',
    label: '硬核死亡后重建角色',
    description: '硬核模式死亡后要求重新创建角色。',
    kind: 'boolean',
  },
  PAL_LOST: {
    group: 'pal',
    label: '死亡丢失帕鲁',
    description: '允许死亡惩罚包含已携带的帕鲁。',
    kind: 'boolean',
  },
  CAN_PICKUP_OTHER_GUILD_DEATH_PENALTY_DROP: {
    group: 'guild',
    label: '拾取其他公会死亡掉落',
    description: '允许玩家拾取其他公会成员的死亡掉落。',
    kind: 'boolean',
  },
  ENABLE_NON_LOGIN_PENALTY: {
    group: 'player',
    label: '离线惩罚',
    description: '允许玩家离线期间发生相关衰减和惩罚。',
    kind: 'boolean',
  },
  ENABLE_FAST_TRAVEL: {
    group: 'world',
    label: '快速传送',
    description: '允许玩家使用快速传送点。',
    kind: 'boolean',
  },
  IS_START_LOCATION_SELECT_BY_MAP: {
    group: 'world',
    label: '地图选择出生点',
    description: '新角色可在地图上选择初始出生位置。',
    kind: 'boolean',
  },
  EXIST_PLAYER_AFTER_LOGOUT: {
    group: 'guild',
    label: '离线后保留角色',
    description: '玩家退出后仍在世界中保留角色实体。',
    kind: 'boolean',
  },
  ENABLE_DEFENSE_OTHER_GUILD_PLAYER: {
    group: 'guild',
    label: '防御其他公会玩家',
    description: '允许针对其他公会玩家的防御行为。',
    kind: 'boolean',
  },
  INVISIBLE_OTHER_GUILD_BASE_CAMP_AREA_FX: {
    group: 'guild',
    label: '隐藏其他公会据点范围',
    description: '不显示其他公会据点的范围效果。',
    kind: 'boolean',
  },
  BUILD_AREA_LIMIT: {
    group: 'building',
    label: '限制建造区域',
    description: '启用服务端建造区域限制。',
    kind: 'boolean',
  },
  ITEM_WEIGHT_RATE: {
    group: 'player',
    label: '物品重量倍率',
    description: '调整背包内物品对负重的影响。',
    kind: 'number',
  },
  COOP_PLAYER_MAX_NUM: {
    group: 'guild',
    label: '合作队伍人数上限',
    description: '限制合作玩法中的玩家数量。',
    kind: 'number',
    step: 1,
  },
  REGION: {
    group: 'advanced',
    label: '服务器区域',
    description: '设置社区服务器列表中的区域标识。',
    kind: 'text',
  },
  USEAUTH: {
    group: 'advanced',
    label: '服务端鉴权',
    description: '启用 Palworld 服务端身份验证。',
    kind: 'boolean',
  },
  BAN_LIST_URL: {
    group: 'advanced',
    label: '封禁列表地址',
    description: '服务端定期读取的远程封禁列表 URL。',
    kind: 'text',
  },
  SHOW_PLAYER_LIST: {
    group: 'advanced',
    label: '显示玩家列表',
    description: '允许客户端查看服务器玩家列表。',
    kind: 'boolean',
  },
  CHAT_POST_LIMIT_PER_MINUTE: {
    group: 'advanced',
    label: '每分钟聊天上限',
    description: '限制单个玩家每分钟发送的聊天消息数。',
    kind: 'number',
    step: 1,
  },
  USE_BACKUP_SAVE_DATA: {
    group: 'world',
    label: '使用备份存档数据',
    description: '世界存档异常时允许游戏使用内置备份数据。',
    kind: 'boolean',
  },
  SUPPLY_DROP_SPAN: {
    group: 'world',
    label: '补给投放间隔',
    description: '控制补给投放事件的时间间隔。',
    kind: 'number',
  },
  ENABLE_PREDATOR_BOSS_PAL: {
    group: 'world',
    label: '掠食者首领帕鲁',
    description: '允许世界生成掠食者首领帕鲁。',
    kind: 'boolean',
  },
  MAX_BUILDING_LIMIT_NUM: {
    group: 'building',
    label: '建筑数量上限',
    description: '限制服务器中的建筑数量；0 使用游戏默认。',
    kind: 'number',
    step: 1,
  },
  SERVER_REPLICATE_PAWN_CULL_DISTANCE: {
    group: 'building',
    label: '实体同步裁剪距离',
    description: '控制远距离角色和实体的网络同步范围。',
    kind: 'number',
  },
  ALLOW_GLOBAL_PALBOX_EXPORT: {
    group: 'pal',
    label: '允许导出全局帕鲁终端',
    description: '允许玩家将帕鲁导出到全局终端。',
    kind: 'boolean',
  },
  ALLOW_GLOBAL_PALBOX_IMPORT: {
    group: 'pal',
    label: '允许导入全局帕鲁终端',
    description: '允许玩家从全局终端导入帕鲁。',
    kind: 'boolean',
  },
  EQUIPMENT_DURABILITY_DAMAGE_RATE: {
    group: 'player',
    label: '装备耐久损耗倍率',
    description: '调整武器和装备的耐久消耗速度。',
    kind: 'number',
  },
  ITEM_CONTAINER_FORCE_MARK_DIRTY_INTERVAL: {
    group: 'building',
    label: '容器强制同步间隔',
    description: '控制物品容器强制标记并同步的间隔。',
    kind: 'number',
  },
  ITEM_CORRUPTION_MULTIPLIER: {
    group: 'building',
    label: '物品腐坏倍率',
    description: '调整食物等限时物品的腐坏速度。',
    kind: 'number',
  },
  PHYSICS_ACTIVE_DROP_ITEM_MAX_NUM: {
    group: 'building',
    label: '物理活动掉落物上限',
    description: '-1 使用游戏默认，其余数值限制活动掉落物数量。',
    kind: 'number',
    step: 1,
  },
  ALLOW_CLIENT_MOD: {
    group: 'advanced',
    label: '允许客户端模组',
    description: '允许安装模组的客户端连接服务器。',
    kind: 'boolean',
  },
  PLAYER_DATA_PAL_STORAGE_UPDATE_CHECK_TICK_INTERVAL: {
    group: 'pal',
    label: '帕鲁存储更新检查间隔',
    description: '控制玩家帕鲁存储数据的检查频率。',
    kind: 'number',
  },
  LOG_FORMAT_TYPE: {
    group: 'advanced',
    label: '游戏日志格式',
    description: '写入 Palworld 配置的日志格式名称。',
    kind: 'text',
  },
  IS_SHOW_JOIN_LEFT_MESSAGE: {
    group: 'advanced',
    label: '显示进出服务器消息',
    description: '玩家加入或离开时在游戏内显示提示。',
    kind: 'boolean',
  },
  MONSTER_FARM_ACTION_SPEED_RATE: {
    group: 'pal',
    label: '牧场帕鲁工作倍率',
    description: '调整牧场中帕鲁执行生产动作的速度。',
    kind: 'number',
  },
  DENY_TECHNOLOGY_LIST: {
    group: 'advanced',
    label: '禁用科技列表',
    description: '填写需要禁用的科技 ID 列表；留空不限制。',
    kind: 'text',
  },
  GUILD_REJOIN_COOLDOWN_MINUTES: {
    group: 'guild',
    label: '重新加入公会冷却分钟',
    description: '退出公会后再次加入公会需要等待的时间。',
    kind: 'number',
    step: 1,
  },
  AUTO_TRANSFER_MASTER_CHECK_INTERVAL_SECONDS: {
    group: 'guild',
    label: '会长转移检查间隔秒',
    description: '检查是否需要自动转移公会会长的频率。',
    kind: 'number',
  },
  AUTO_TRANSFER_MASTER_THRESHOLD_DAYS: {
    group: 'guild',
    label: '会长转移离线天数',
    description: '会长离线达到该天数后允许自动转移。',
    kind: 'number',
    step: 1,
  },
  MAX_GUILDS_PER_FRAME: {
    group: 'guild',
    label: '每帧处理公会数',
    description: '限制服务器单帧处理的公会数量。',
    kind: 'number',
    step: 1,
  },
  BLOCK_RESPAWN_TIME: {
    group: 'world',
    label: '区块重生时间',
    description: '控制世界区块内容重新生成的等待时间。',
    kind: 'number',
  },
  RESPAWN_PENALTY_DURATION_THRESHOLD: {
    group: 'player',
    label: '复活惩罚触发时长',
    description: '达到该持续时间后应用复活惩罚倍率。',
    kind: 'number',
  },
  RESPAWN_PENALTY_TIME_SCALE: {
    group: 'player',
    label: '复活惩罚时间倍率',
    description: '调整复活惩罚持续时间。',
    kind: 'number',
  },
  DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_BASE_CAMP: {
    group: 'guild',
    label: '地图显示据点 PvP 物资',
    description: '在世界地图显示据点持有的 PvP 物资数量。',
    kind: 'boolean',
  },
  DISPLAY_PVP_ITEM_NUM_ON_WORLD_MAP_PLAYER: {
    group: 'guild',
    label: '地图显示玩家 PvP 物资',
    description: '在世界地图显示玩家持有的 PvP 物资数量。',
    kind: 'boolean',
  },
  ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE: {
    group: 'guild',
    label: 'PvP 击杀额外掉落物',
    description: '设置 PvP 击杀玩家时额外生成的物品 ID。',
    kind: 'text',
  },
  ADDITIONAL_DROP_ITEM_NUM_WHEN_PLAYER_KILLING_IN_PVP_MODE: {
    group: 'guild',
    label: 'PvP 击杀额外掉落数量',
    description: '设置 PvP 击杀额外掉落物的数量。',
    kind: 'number',
    step: 1,
  },
  ADDITIONAL_DROP_ITEM_WHEN_PLAYER_KILLING_IN_PVP_MODE_ENABLED: {
    group: 'guild',
    label: '启用 PvP 击杀额外掉落',
    description: '允许 PvP 击杀玩家时生成指定额外物品。',
    kind: 'boolean',
  },
  ENABLE_VOICE_CHAT: {
    group: 'player',
    label: '语音聊天',
    description: '允许玩家使用游戏内距离语音。',
    kind: 'boolean',
  },
  VOICE_CHAT_MAX_VOLUME_DISTANCE: {
    group: 'player',
    label: '语音最大音量距离',
    description: '距离小于该值时保持最大语音音量。',
    kind: 'number',
  },
  VOICE_CHAT_ZERO_VOLUME_DISTANCE: {
    group: 'player',
    label: '语音静音距离',
    description: '距离超过该值后听不到语音。',
    kind: 'number',
  },
  ALLOW_ENHANCE_STAT_HEALTH: {
    group: 'player',
    label: '允许强化生命',
    description: '允许玩家强化生命属性。',
    kind: 'boolean',
  },
  ALLOW_ENHANCE_STAT_ATTACK: {
    group: 'player',
    label: '允许强化攻击',
    description: '允许玩家强化攻击属性。',
    kind: 'boolean',
  },
  ALLOW_ENHANCE_STAT_STAMINA: {
    group: 'player',
    label: '允许强化耐力',
    description: '允许玩家强化耐力属性。',
    kind: 'boolean',
  },
  ALLOW_ENHANCE_STAT_WEIGHT: {
    group: 'player',
    label: '允许强化负重',
    description: '允许玩家强化负重属性。',
    kind: 'boolean',
  },
  ALLOW_ENHANCE_STAT_WORK_SPEED: {
    group: 'player',
    label: '允许强化工作速度',
    description: '允许玩家强化工作速度属性。',
    kind: 'boolean',
  },
  ENABLE_BUILDING_PLAYER_UID_DISPLAY: {
    group: 'building',
    label: '显示建筑所属玩家 UID',
    description: '在建筑信息中显示建造者玩家 UID。',
    kind: 'boolean',
  },
  BUILDING_NAME_DISPLAY_CACHE_TTL_SECONDS: {
    group: 'building',
    label: '建筑名称缓存秒数',
    description: '控制建筑名称显示数据的缓存时长。',
    kind: 'number',
  },
} satisfies Record<GameParameterName, ParameterDefinition>

const PARAMETER_ENTRIES = Object.entries(PARAMETER_DEFINITIONS) as Array<[
  GameParameterName,
  ParameterDefinition,
]>

const isTrue = (value: string) => value.toLowerCase() === 'true'

export function GameParameterFields({ value, onChange }: GameParameterFieldsProps) {
  const [activeGroup, setActiveGroup] = useState<ParameterGroup>('world')
  const group = PARAMETER_GROUPS.find((item) => item.key === activeGroup) ?? PARAMETER_GROUPS[0]
  const fields = PARAMETER_ENTRIES.filter(([, definition]) => definition.group === activeGroup)

  const update = (key: GameParameterName, nextValue: string) => {
    onChange({ ...value, [key]: nextValue })
  }

  return (
    <div className="space-y-6">
      <ScrollableTabBar
        tabs={PARAMETER_GROUPS}
        activeTab={activeGroup}
        onTabChange={setActiveGroup}
        indicatorId="palworld-game-parameter-groups"
        outerClassName="[-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
      />

      <div className="flex flex-wrap items-end justify-between gap-2">
        <div className="space-y-1">
          <h3 className="text-sm font-semibold text-foreground">{group.label}</h3>
          <p className="text-sm text-muted-foreground">{group.description}</p>
        </div>
        <span className="text-xs text-muted-foreground">{fields.length} 项</span>
      </div>

      <div className="grid items-start gap-x-8 gap-y-5 lg:grid-cols-2">
        {fields.map(([key, definition]) => {
          const inputId = `game-parameter-${key.toLowerCase().replace(/_/g, '-')}`
          const currentValue = value[key] ?? DEFAULT_GAME_PARAMETERS[key]

          if (definition.kind === 'boolean') {
            return (
              <div key={key} className="flex min-h-24 items-start justify-between gap-5 border-b border-border/70 pb-5">
                <div className="min-w-0 space-y-1">
                  <Label htmlFor={inputId}>{definition.label}</Label>
                  <p className="text-xs leading-5 text-muted-foreground">{definition.description}</p>
                  <code className="block break-all text-[11px] leading-5 text-muted-foreground/80">{key}</code>
                </div>
                <Switch
                  id={inputId}
                  className="mt-0.5 shrink-0"
                  checked={isTrue(currentValue)}
                  onCheckedChange={(checked) => update(key, checked ? 'True' : 'False')}
                />
              </div>
            )
          }

          return (
            <div key={key} className="min-h-32 space-y-2 border-b border-border/70 pb-5">
              <div className="space-y-1">
                <Label htmlFor={definition.kind === 'select' ? undefined : inputId}>{definition.label}</Label>
                <p className="text-xs leading-5 text-muted-foreground">{definition.description}</p>
                <code className="block break-all text-[11px] leading-5 text-muted-foreground/80">{key}</code>
              </div>
              {definition.kind === 'select' ? (
                <Combobox
                  options={definition.options ?? []}
                  value={currentValue}
                  onValueChange={(nextValue) => update(key, nextValue)}
                  searchPlaceholder={`搜索${definition.label}...`}
                />
              ) : (
                <Input
                  id={inputId}
                  type={definition.kind === 'number' ? 'number' : 'text'}
                  inputMode={definition.kind === 'number' ? 'decimal' : undefined}
                  step={definition.kind === 'number' ? definition.step ?? 'any' : undefined}
                  spellCheck={definition.kind === 'text' ? false : undefined}
                  value={currentValue}
                  onChange={(event) => update(key, event.target.value)}
                />
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
