import { config } from './config/env.mjs'
import { openDatabase, createRepositories } from './db/index.mjs'
import { buildApp } from './app.mjs'

const db = openDatabase(config)
const repos = createRepositories(db)
repos.db = db

const app = await buildApp({ config, repos })

await app.listen({ host: config.bind, port: config.port })
