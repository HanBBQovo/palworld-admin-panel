import { execFile } from 'node:child_process'

export function run(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    execFile(command, args, { timeout: options.timeout || 15000, cwd: options.cwd }, (err, stdout, stderr) => {
      if (err) {
        err.stdout = stdout
        err.stderr = stderr
        reject(err)
        return
      }
      resolve({ stdout: stdout.trim(), stderr: stderr.trim() })
    })
  })
}

export async function safeRun(command, args, options = {}, fallback = null) {
  try {
    return await run(command, args, options)
  } catch {
    return fallback
  }
}

export function docker(config, args, options = {}) {
  return run('docker', args, { timeout: options.timeout || 20000, cwd: options.cwd || config.composeDir })
}

export async function safeDocker(config, args, fallback = null) {
  try {
    return await docker(config, args)
  } catch {
    return fallback
  }
}
