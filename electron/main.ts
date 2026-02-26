import { app, BrowserWindow, ipcMain, dialog, shell } from 'electron'
import { spawn, ChildProcess } from 'child_process'
import * as path from 'path'

let mainWindow: BrowserWindow | null = null
let goProcess: ChildProcess | null = null
let serverUrl: string = ''
let bindAll: boolean = false

const isDev = process.env.NODE_ENV === 'development' || !app.isPackaged

function getGoBinaryPath(): string {
  if (isDev) {
    return path.join(__dirname, '..', 'bore-server')
  }
  const ext = process.platform === 'win32' ? '.exe' : ''
  return path.join(process.resourcesPath, `bore-server${ext}`)
}

function startGoServer(networkMode: boolean = false): Promise<string> {
  return new Promise((resolve, reject) => {
    const binaryPath = getGoBinaryPath()
    const args: string[] = []
    if (networkMode) {
      args.push('--bind', '0.0.0.0')
    }

    goProcess = spawn(binaryPath, args, {
      stdio: ['ignore', 'pipe', 'pipe'],
    })

    let resolved = false
    const timeout = setTimeout(() => {
      if (!resolved) reject(new Error('Go server startup timed out after 30s'))
    }, 30000)

    goProcess.stdout?.on('data', (data: Buffer) => {
      const lines = data.toString().split('\n')
      for (const line of lines) {
        const match = line.match(/^BORE_SERVER_URL=(.+)$/)
        if (match && !resolved) {
          resolved = true
          clearTimeout(timeout)
          serverUrl = match[1].trim()
          resolve(serverUrl)
        }
      }
    })

    goProcess.stderr?.on('data', (data: Buffer) => {
      const msg = data.toString()
      process.stderr.write(msg)
      if (!msg.endsWith('\n')) process.stderr.write('\n')
    })

    goProcess.on('error', (err) => {
      if (!resolved) {
        resolved = true
        clearTimeout(timeout)
        reject(err)
      }
    })

    goProcess.on('exit', (code) => {
      console.log(`bore-server exited with code ${code}`)
    })
  })
}

function stopGoServer(): Promise<void> {
  return new Promise((resolve) => {
    if (!goProcess) {
      resolve()
      return
    }
    let settled = false
    const done = () => {
      if (!settled) {
        settled = true
        clearTimeout(forceKillTimer)
        goProcess = null
        resolve()
      }
    }
    goProcess.on('exit', done)
    goProcess.kill('SIGTERM')
    const forceKillTimer = setTimeout(() => {
      goProcess?.kill('SIGKILL')
      done()
    }, 5000)
  })
}

async function createWindow(url: string): Promise<void> {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    title: 'BORE',
    backgroundColor: '#09090b',
    icon: path.join(__dirname, '..', 'assets', 'icon.png'),
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  })

  mainWindow.setMenuBarVisibility(false)

  await mainWindow.loadURL(url)

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

// IPC Handlers
ipcMain.handle('get-api-url', () => serverUrl)

ipcMain.handle('pick-folder', async () => {
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ['openDirectory'],
    title: 'Select Project Folder',
  })
  return result.canceled ? null : result.filePaths[0]
})

ipcMain.handle('toggle-network', async (_event, enable: boolean) => {
  const previousBindAll = bindAll
  const oldUrl = serverUrl

  // Stop old server
  await stopGoServer()

  // Start new server with/without network mode
  try {
    const newUrl = await startGoServer(enable)
    bindAll = enable
    serverUrl = newUrl
    // Reload the window to the new URL
    if (mainWindow) {
      await mainWindow.loadURL(newUrl)
    }
    return { ok: true, url: newUrl }
  } catch (err) {
    // Restart with previous mode on failure
    try {
      const recoveredUrl = await startGoServer(previousBindAll)
      serverUrl = recoveredUrl
      if (mainWindow) {
        await mainWindow.loadURL(recoveredUrl)
      }
    } catch {
      // Recovery also failed; serverUrl stays stale, caller handles it
    }
    return { ok: false, error: String(err) }
  }
})

ipcMain.handle('open-external', (_event, url: string) => {
  return shell.openExternal(url)
})

app.whenReady().then(async () => {
  // Set macOS dock icon explicitly (needed in dev/unpackaged mode)
  if (process.platform === 'darwin' && app.dock) {
    app.dock.setIcon(path.join(__dirname, '..', 'assets', 'icon.png'))
  }

  try {
    const url = await startGoServer()
    await createWindow(url)
  } catch (err) {
    console.error('Failed to start bore-server:', err)
    app.quit()
  }
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('activate', async () => {
  if (BrowserWindow.getAllWindows().length === 0 && serverUrl) {
    await createWindow(serverUrl)
  }
})

app.on('before-quit', async (event) => {
  event.preventDefault()
  await stopGoServer()
  app.exit(0)
})
