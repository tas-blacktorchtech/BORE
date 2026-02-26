import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Wifi, WifiOff, Server, Smartphone, Copy, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import QRCode from 'qrcode'
import { api, getBaseUrl } from '@/lib/api'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'

export function Settings() {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [networkToggling, setNetworkToggling] = useState(false)

  const { data: serverInfo, isLoading, refetch } = useQuery({
    queryKey: ['server-info'],
    queryFn: api.server.info,
    refetchInterval: 5000,
  })

  const isElectron = !!window.electronAPI?.isElectron
  const isNetworkMode = serverInfo?.network_mode ?? false

  // Generate QR code for mobile access
  useEffect(() => {
    if (!canvasRef.current || !serverInfo?.local_ips?.length || !isNetworkMode) return

    const ip = serverInfo.local_ips[0]
    const port = serverInfo.port
    const url = `http://${ip}:${port}`

    QRCode.toCanvas(canvasRef.current, url, {
      width: 200,
      color: {
        dark: '#e4e4e7',  // zinc-200
        light: '#09090b', // zinc-950
      },
    }).catch(err => {
      console.error('QR generation failed:', err)
    })
  }, [serverInfo, isNetworkMode])

  const handleToggleNetwork = async () => {
    if (!isElectron) {
      toast.error('Network mode toggle requires the Electron app')
      return
    }
    setNetworkToggling(true)
    try {
      await window.electronAPI!.toggleNetwork!(!isNetworkMode)
      await refetch()
      toast.success(isNetworkMode ? 'Network mode disabled' : 'Network mode enabled')
    } catch (err) {
      toast.error('Failed to toggle network mode')
    } finally {
      setNetworkToggling(false)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      toast.success('Copied to clipboard')
    }).catch(() => {
      toast.error('Failed to copy')
    })
  }

  return (
    <div className="p-6 space-y-6 max-w-2xl mx-auto">
      <h1 className="text-2xl font-bold text-zinc-100">Settings</h1>

      {/* Server Info */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Server className="h-4 w-4 text-zinc-400" />
            Server Info
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {isLoading ? (
            <div className="flex items-center gap-2 text-zinc-500 text-sm">
              <RefreshCw className="h-3 w-3 animate-spin" />
              Loading...
            </div>
          ) : serverInfo ? (
            <>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <div className="text-xs text-zinc-500 mb-1">Port</div>
                  <div className="font-mono text-sm text-zinc-200">{serverInfo.port}</div>
                </div>
                <div>
                  <div className="text-xs text-zinc-500 mb-1">Bind Address</div>
                  <div className="font-mono text-sm text-zinc-200">{serverInfo.bind}</div>
                </div>
              </div>
              <div>
                <div className="text-xs text-zinc-500 mb-1">API Base URL</div>
                <div className="flex items-center gap-2">
                  <div className="font-mono text-sm text-zinc-200 flex-1">{getBaseUrl()}</div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => copyToClipboard(getBaseUrl())}
                  >
                    <Copy className="h-3 w-3" />
                  </Button>
                </div>
              </div>
            </>
          ) : (
            <p className="text-sm text-zinc-500">Unable to load server info</p>
          )}
        </CardContent>
      </Card>

      {/* Network Access */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Wifi className="h-4 w-4 text-zinc-400" />
            Network Access
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Toggle */}
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-zinc-200">Network Mode</div>
              <div className="text-xs text-zinc-500 mt-0.5">
                {isNetworkMode
                  ? 'API is accessible from other devices on your network'
                  : 'API is only accessible from localhost'}
              </div>
            </div>
            <button
              onClick={handleToggleNetwork}
              disabled={networkToggling || !isElectron}
              className={cn(
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none disabled:opacity-50 disabled:cursor-not-allowed',
                isNetworkMode ? 'bg-green-600' : 'bg-zinc-700'
              )}
              title={!isElectron ? 'Requires Electron app' : undefined}
            >
              <span
                className={cn(
                  'inline-block h-4 w-4 rounded-full bg-white transition-transform',
                  isNetworkMode ? 'translate-x-6' : 'translate-x-1'
                )}
              />
            </button>
          </div>

          {!isElectron && (
            <div className="flex items-center gap-2 rounded-md bg-zinc-900 border border-zinc-800 px-3 py-2">
              <WifiOff className="h-4 w-4 text-zinc-500 shrink-0" />
              <p className="text-xs text-zinc-500">
                Network mode toggle requires the Electron app. Start BORE via the desktop app to enable this feature.
              </p>
            </div>
          )}

          {/* Local IPs when network mode is on */}
          {isNetworkMode && serverInfo?.local_ips && serverInfo.local_ips.length > 0 && (
            <>
              <Separator />
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <Smartphone className="h-4 w-4 text-zinc-400" />
                  <span className="text-sm font-medium text-zinc-200">Access from other devices</span>
                </div>
                <div className="space-y-2">
                  {serverInfo.local_ips.map(ip => {
                    const url = `http://${ip}:${serverInfo.port}`
                    return (
                      <div
                        key={ip}
                        className="flex items-center gap-2 rounded-md bg-zinc-900 border border-zinc-800 px-3 py-2"
                      >
                        <div className="font-mono text-sm text-zinc-200 flex-1">{url}</div>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => copyToClipboard(url)}
                        >
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                    )
                  })}
                </div>
              </div>

              {/* QR Code */}
              <div>
                <div className="text-xs text-zinc-500 mb-3">
                  Scan to open on your phone or tablet:
                </div>
                <div className="inline-block rounded-lg border border-zinc-800 bg-zinc-950 p-3">
                  <canvas ref={canvasRef} />
                </div>
                <div className="text-xs text-zinc-600 mt-2 font-mono">
                  {`http://${serverInfo.local_ips[0]}:${serverInfo.port}`}
                </div>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* App Info */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">About BORE</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <div className="grid grid-cols-2 gap-2 text-sm">
            <div className="text-zinc-500">App</div>
            <div className="text-zinc-200">BORE — AI Task Orchestration</div>
            <div className="text-zinc-500">Runtime</div>
            <div className="text-zinc-200">{isElectron ? 'Electron (Desktop)' : 'Web Browser'}</div>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
