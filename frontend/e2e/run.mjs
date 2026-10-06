import { spawn } from "node:child_process"
import { randomBytes, randomUUID } from "node:crypto"
import { createRequire } from "node:module"
import { createServer } from "node:net"
import { fileURLToPath } from "node:url"

const frontendDir = fileURLToPath(new URL("../", import.meta.url))
const composeFile = fileURLToPath(new URL("./compose.yml", import.meta.url))
const project = `task2todaytodo-e2e-${randomUUID()}`
const env = { ...process.env, E2E_PAGE_TOKEN_KEY: randomBytes(32).toString("base64") }
const require = createRequire(import.meta.url)
let activeProcess
let interrupted = 0
let cleaningUp = false

for (const [signal, exitCode] of [["SIGINT", 130], ["SIGTERM", 143]]) {
  process.on(signal, () => {
    interrupted = exitCode
    if (!cleaningUp) activeProcess?.kill("SIGINT")
  })
}

function run(command, args, capture = false) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: frontendDir,
      env,
      stdio: capture ? ["ignore", "pipe", "inherit"] : "inherit",
    })
    activeProcess = child
    let output = ""
    child.stdout?.on("data", (chunk) => { output += chunk })
    child.once("error", reject)
    child.once("close", (code, signal) => {
      activeProcess = undefined
      if (code === 0) resolve(output.trim())
      else reject(new Error(`${command} exited with ${code ?? signal}`))
    })
  })
}

function compose(args, capture = false) {
  return run("docker", ["compose", "--project-name", project, "--file", composeFile, ...args], capture)
}

async function availablePort() {
  const server = createServer()
  await new Promise((resolve, reject) => {
    server.once("error", reject)
    server.listen(0, "127.0.0.1", resolve)
  })
  const { port } = server.address()
  await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
  return port
}

console.log(`E2E environment: ${project}`)
try {
  await compose(["up", "--build", "--wait", "--wait-timeout", "120"])
  if (interrupted) throw new Error("E2E interrupted")
  const address = await compose(["port", "api", "8080"], true)
  env.E2E_API_URL = `http://${address}`
  env.E2E_FRONTEND_PORT = String(await availablePort())
  if (interrupted) throw new Error("E2E interrupted")
  await run(process.execPath, [require.resolve("@playwright/test/cli"), "test", ...process.argv.slice(2)])
} catch (error) {
  process.exitCode = interrupted || 1
  console.error(error.message)
  if (!interrupted) {
    await compose(["logs", "--no-color", "--tail", "60"]).catch(() => {})
  }
} finally {
  cleaningUp = true
  await compose(["down", "--volumes", "--remove-orphans", "--rmi", "local"]).catch((error) => {
    process.exitCode = interrupted || 1
    console.error(`E2E cleanup failed: ${error.message}`)
    console.error(`Cleanup command: E2E_PAGE_TOKEN_KEY=unused docker compose -p ${project} -f ${composeFile} down --volumes --rmi local`)
  })
  if (interrupted) process.exitCode = interrupted
}
