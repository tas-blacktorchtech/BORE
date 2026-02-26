import { contextBridge, ipcRenderer } from 'electron'

contextBridge.exposeInMainWorld('electronAPI', {
  pickFolder: (): Promise<string | null> =>
    ipcRenderer.invoke('pick-folder'),

  toggleNetwork: (enable: boolean): Promise<{ ok: boolean; url?: string; error?: string }> =>
    ipcRenderer.invoke('toggle-network', enable),

  getApiUrl: (): Promise<string> =>
    ipcRenderer.invoke('get-api-url'),

  openExternal: (url: string): Promise<void> =>
    ipcRenderer.invoke('open-external', url),

  isElectron: true,
})
