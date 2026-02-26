/// <reference types="vite/client" />

interface ElectronAPI {
  pickFolder: () => Promise<string | null>
  toggleNetwork: (enable: boolean) => Promise<{ ok: boolean; url?: string; error?: string }>
  getApiUrl: () => Promise<string>
  openExternal: (url: string) => Promise<void>
  isElectron: boolean
}

interface Window {
  electronAPI?: ElectronAPI
}
