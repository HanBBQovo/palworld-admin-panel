import { AlertCircle, Archive, RefreshCw } from 'lucide-react'

import type { WorldStatus } from '@/api/palworld'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

export function WorldIndexAlert({ status }: { status: WorldStatus | null | undefined }) {
  if (!status) return null

  if (status.indexSyncing) {
    return (
      <Alert>
        <RefreshCw className="animate-spin" />
        <AlertTitle>正在解析最新备份</AlertTitle>
        <AlertDescription>
          世界索引正在读取玩家、公会和基地数据，完成前页面不会把旧结果标记为最新。
        </AlertDescription>
      </Alert>
    )
  }

  if (status.indexStale || status.indexLastError) {
    return (
      <Alert variant="destructive">
        <AlertCircle />
        <AlertTitle>世界索引解析失败，当前数据不是最新</AlertTitle>
        <AlertDescription>
          {status.indexLastError || '索引服务返回的指纹与当前快照不一致。请重新读取最新备份。'}
        </AlertDescription>
      </Alert>
    )
  }

  if (!status.upToDate) {
    return (
      <Alert>
        <Archive />
        <AlertTitle>世界索引正在追赶最新备份</AlertTitle>
        <AlertDescription>
          当前索引 {status.snapshot.backupId || '-'}，最新备份 {status.latestBackupId || '-'}。
        </AlertDescription>
      </Alert>
    )
  }

  return null
}
