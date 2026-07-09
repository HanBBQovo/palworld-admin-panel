import { FormEvent, useState } from 'react'
import { Gamepad2, Loader2, LogIn, ShieldCheck } from 'lucide-react'

import { login } from '@/api/auth'
import { ThemeToggleButton } from '@/components/theme'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { BRAND_NAME } from '@/config'
import { motion } from '@/lib/motion'

interface LoginProps {
  onAuthenticated: () => void
}

export default function Login({ onAuthenticated }: LoginProps) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await login(password)
      onAuthenticated()
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-bg flex min-h-screen items-center justify-center px-4 py-10">
      <div className="fixed right-4 top-4 z-10">
        <ThemeToggleButton />
      </div>
      <motion.form
        onSubmit={handleSubmit}
        className="glass-card w-full max-w-sm rounded-2xl p-6"
        initial={{ opacity: 0, y: 18, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', bounce: 0.18, duration: 0.45 }}
      >
        <div className="mb-6 flex items-center gap-3">
          <div className="dashboard-brand-mark flex h-10 w-10 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Gamepad2 className="h-5 w-5" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{BRAND_NAME}</h1>
            <p className="text-sm text-muted-foreground">Palworld 服务器管理面板</p>
          </div>
        </div>

        <div className="mb-5 rounded-xl border border-border/70 bg-background/60 p-3 text-sm text-muted-foreground">
          <div className="mb-1 flex items-center gap-2 font-medium text-foreground">
            <ShieldCheck className="h-4 w-4 text-primary" />
            管理入口只面向运维人员
          </div>
          登录后可执行重启、恢复备份、保存配置等高风险操作。
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="password">面板密码</Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoFocus
            autoComplete="current-password"
          />
        </div>

        {error ? <div className="mt-3 text-sm text-destructive">{error}</div> : null}

        <Button type="submit" className="mt-5 w-full gap-2" disabled={submitting || !password}>
          {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
          登录管理面板
        </Button>
      </motion.form>
    </div>
  )
}
