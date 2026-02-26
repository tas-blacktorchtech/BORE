import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { FolderOpen, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

export function Home() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [newPath, setNewPath] = useState('')
  const [newName, setNewName] = useState('')
  const [clusterToDelete, setClusterToDelete] = useState<string | null>(null)

  const { data: status } = useQuery({
    queryKey: ['status'],
    queryFn: api.status.get,
    refetchInterval: 3000,
  })

  const { data: clusters, refetch: refetchClusters } = useQuery({
    queryKey: ['clusters-list'],
    queryFn: api.clusters.list,
  })

  const openMutation = useMutation({
    mutationFn: (path: string) => api.clusters.open(path),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['status'] })
      toast.success('Cluster opened')
      navigate('/dashboard')
    },
    onError: (e: Error) => toast.error(`Failed to open: ${e.message}`),
  })

  const initMutation = useMutation({
    mutationFn: ({ path, name }: { path: string; name: string }) =>
      api.clusters.init(path, name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['status'] })
      queryClient.invalidateQueries({ queryKey: ['clusters-list'] })
      toast.success('Cluster created and opened')
      setShowCreateDialog(false)
      navigate('/dashboard')
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  })

  const deleteMutation = useMutation({
    mutationFn: (path: string) => api.clusters.delete(path),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['clusters-list'] })
      queryClient.invalidateQueries({ queryKey: ['status'] })
      toast.success('Cluster deleted')
      setClusterToDelete(null)
    },
    onError: (e: Error) => {
      toast.error(`Failed to delete: ${e.message}`)
      setClusterToDelete(null)
    },
  })

  // Redirect if cluster already open
  useEffect(() => {
    if (status?.has_cluster) {
      navigate('/dashboard')
    }
  }, [status?.has_cluster, navigate])

  const pickFolder = async () => {
    if (window.electronAPI?.pickFolder) {
      const path = await window.electronAPI.pickFolder()
      if (path) setNewPath(path)
    }
  }

  return (
    <div className="flex flex-col items-center justify-center h-full min-h-[80vh] p-8">
      {/* Logo */}
      <div className="mb-12 text-center">
        <img
          src="/borelogo.png"
          alt="BORE"
          className="mx-auto mb-6 h-52 w-auto object-contain select-none"
          draggable={false}
        />
        <p className="text-zinc-400 text-lg">AI-powered development task orchestration</p>
      </div>

      {/* Known clusters */}
      {clusters && clusters.length > 0 && (
        <div className="w-full max-w-2xl mb-8">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-medium text-zinc-400 uppercase tracking-wider">Known Clusters</h2>
            <Button variant="ghost" size="sm" onClick={() => refetchClusters()}>
              <RefreshCw className="h-3 w-3" />
            </Button>
          </div>
          <div className="grid gap-3">
            {clusters.map(cluster => (
              <Card
                key={cluster.path}
                className="cursor-pointer hover:border-zinc-600 transition-colors"
                onClick={() => openMutation.mutate(cluster.path)}
              >
                <CardContent className="flex items-center justify-between p-4">
                  <div className="min-w-0 flex-1">
                    <div className="font-medium text-zinc-100">{cluster.name}</div>
                    <div className="text-xs text-zinc-500 mt-1 font-mono truncate">{cluster.path}</div>
                  </div>
                  <div className="flex items-center gap-2 ml-4 shrink-0">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-zinc-500 hover:text-red-400"
                      onClick={(e) => {
                        e.stopPropagation()
                        setClusterToDelete(cluster.path)
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                    <FolderOpen className="h-4 w-4 text-zinc-400" />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="flex gap-4">
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="h-4 w-4" />
          New Cluster
        </Button>
        {clusters && clusters.length === 0 && (
          <Button variant="outline" onClick={() => refetchClusters()}>
            <RefreshCw className="h-4 w-4" />
            Refresh
          </Button>
        )}
      </div>

      {/* Create dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create New Cluster</DialogTitle>
            <DialogDescription>Set up BORE in a project directory</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="cluster-name">Cluster Name</Label>
              <Input
                id="cluster-name"
                value={newName}
                onChange={e => setNewName(e.target.value)}
                placeholder="my-project"
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="cluster-path">Project Path</Label>
              <div className="flex gap-2 mt-1">
                <Input
                  id="cluster-path"
                  value={newPath}
                  onChange={e => setNewPath(e.target.value)}
                  placeholder="/path/to/project"
                  className="flex-1"
                />
                {window.electronAPI?.isElectron && (
                  <Button variant="outline" size="icon" onClick={pickFolder} title="Browse">
                    <FolderOpen className="h-4 w-4" />
                  </Button>
                )}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowCreateDialog(false)}>Cancel</Button>
            <Button
              onClick={() => initMutation.mutate({ path: newPath, name: newName })}
              disabled={!newPath || !newName || initMutation.isPending}
            >
              {initMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!clusterToDelete} onOpenChange={(open) => { if (!open) setClusterToDelete(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Cluster</AlertDialogTitle>
            <AlertDialogDescription>
              This will remove the .bore/ directory and all task history for this cluster.
              Your project files will not be affected. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-red-600 hover:bg-red-700"
              onClick={() => clusterToDelete && deleteMutation.mutate(clusterToDelete)}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
