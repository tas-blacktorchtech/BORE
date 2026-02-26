import { useState, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  GitBranch,
  Clock,
  CheckCircle,
  XCircle,
  AlertTriangle,
  RefreshCw,
  GitCommit,
  RotateCcw,
  GitMerge,
} from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { cn, timeAgo, formatDate } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { TaskStatus, LogLevel } from '@/lib/types'

const STATUS_STYLES: Record<TaskStatus, string> = {
  pending: 'bg-zinc-700 text-zinc-300',
  review: 'bg-yellow-900/60 text-yellow-300',
  running: 'bg-blue-900/60 text-blue-300 animate-pulse',
  diff_review: 'bg-purple-900/60 text-purple-300',
  completed: 'bg-green-900/60 text-green-300',
  failed: 'bg-red-900/60 text-red-300',
  interrupted: 'bg-orange-900/60 text-orange-300',
}

const LEVEL_COLORS: Record<LogLevel, string> = {
  debug: 'text-zinc-500',
  info: 'text-blue-400',
  warn: 'text-yellow-400',
  error: 'text-red-400',
}

function DiffView({ executionId }: { executionId: number }) {
  const queryClient = useQueryClient()
  const [commitMsg, setCommitMsg] = useState('')
  const [confirmAction, setConfirmAction] = useState<'commit' | 'revert' | 'merge' | null>(null)

  const { data: diffData, isLoading, refetch } = useQuery({
    queryKey: ['diff', executionId],
    queryFn: () => api.diff.get(executionId),
  })

  const commitMutation = useMutation({
    mutationFn: () => api.diff.commit(executionId, commitMsg || undefined),
    onSuccess: () => {
      toast.success('Changes committed')
      setConfirmAction(null)
      queryClient.invalidateQueries({ queryKey: ['executions'] })
      refetch()
    },
    onError: (e: Error) => toast.error(`Commit failed: ${e.message}`),
  })

  const revertMutation = useMutation({
    mutationFn: () => api.diff.revert(executionId),
    onSuccess: () => {
      toast.success('Changes reverted')
      setConfirmAction(null)
      queryClient.invalidateQueries({ queryKey: ['executions'] })
      refetch()
    },
    onError: (e: Error) => toast.error(`Revert failed: ${e.message}`),
  })

  const mergeMutation = useMutation({
    mutationFn: () => api.diff.merge(executionId),
    onSuccess: data => {
      toast.success(data.message || 'Merged to base branch')
      setConfirmAction(null)
      queryClient.invalidateQueries({ queryKey: ['executions'] })
      refetch()
    },
    onError: (e: Error) => toast.error(`Merge failed: ${e.message}`),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-16 text-zinc-500">
        <RefreshCw className="h-4 w-4 animate-spin mr-2" />
        Loading diff...
      </div>
    )
  }

  if (!diffData) {
    return <div className="py-8 text-center text-zinc-500">No diff data available</div>
  }

  const renderDiff = (diff: string) => {
    if (!diff.trim()) {
      return <span className="text-zinc-500">No changes detected</span>
    }
    return diff.split('\n').map((line, i) => {
      let lineClass = 'text-zinc-400'
      if (line.startsWith('+') && !line.startsWith('+++')) lineClass = 'text-green-400 bg-green-900/10'
      else if (line.startsWith('-') && !line.startsWith('---')) lineClass = 'text-red-400 bg-red-900/10'
      else if (line.startsWith('@@')) lineClass = 'text-cyan-400'
      else if (line.startsWith('diff ') || line.startsWith('index ') || line.startsWith('---') || line.startsWith('+++')) lineClass = 'text-zinc-300 font-medium'
      return (
        <div key={i} className={cn('font-mono text-xs leading-5 px-2', lineClass)}>
          {line || ' '}
        </div>
      )
    })
  }

  return (
    <div className="space-y-4">
      {/* Status banner */}
      <div className="flex items-center justify-between">
        <div className="text-sm text-zinc-400">
          Status: <span className="text-zinc-100 font-medium">{diffData.status}</span>
        </div>
        <Button variant="ghost" size="sm" onClick={() => refetch()}>
          <RefreshCw className="h-3 w-3" />
        </Button>
      </div>

      {/* Action buttons */}
      {confirmAction === null ? (
        <div className="flex gap-3">
          <Button
            size="sm"
            onClick={() => setConfirmAction('commit')}
            className="bg-green-800 hover:bg-green-700 text-green-100"
          >
            <GitCommit className="h-4 w-4" />
            Commit Changes
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setConfirmAction('merge')}
          >
            <GitMerge className="h-4 w-4" />
            Merge to Base
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => setConfirmAction('revert')}
            className="border-red-800 text-red-400 hover:bg-red-900/20"
          >
            <RotateCcw className="h-4 w-4" />
            Revert Changes
          </Button>
        </div>
      ) : (
        <div className="p-4 rounded-md border border-zinc-700 bg-zinc-900 space-y-3">
          {confirmAction === 'commit' && (
            <div>
              <p className="text-sm text-zinc-300 mb-2">Enter a commit message (optional):</p>
              <input
                type="text"
                value={commitMsg}
                onChange={e => setCommitMsg(e.target.value)}
                placeholder="feat: implement changes from BORE task"
                className="w-full px-3 py-2 text-sm rounded-md border border-input bg-transparent text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          )}
          <p className="text-sm text-zinc-400">
            {confirmAction === 'commit' && 'Are you sure you want to commit these changes?'}
            {confirmAction === 'revert' && 'Are you sure you want to revert all changes? This cannot be undone.'}
            {confirmAction === 'merge' && 'Are you sure you want to merge the execution branch into the base branch?'}
          </p>
          <div className="flex gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={() => setConfirmAction(null)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={() => {
                if (confirmAction === 'commit') commitMutation.mutate()
                else if (confirmAction === 'revert') revertMutation.mutate()
                else if (confirmAction === 'merge') mergeMutation.mutate()
              }}
              disabled={
                commitMutation.isPending ||
                revertMutation.isPending ||
                mergeMutation.isPending
              }
              className={confirmAction === 'revert' ? 'bg-red-800 hover:bg-red-700 text-red-100' : ''}
            >
              {commitMutation.isPending || revertMutation.isPending || mergeMutation.isPending
                ? 'Processing...'
                : 'Confirm'}
            </Button>
          </div>
        </div>
      )}

      {/* Diff content */}
      <div className="rounded-md border border-zinc-800 bg-zinc-950 overflow-hidden">
        <ScrollArea className="h-[500px]">
          <div className="p-0">{renderDiff(diffData.diff)}</div>
        </ScrollArea>
      </div>
    </div>
  )
}

function EventLog({ executionId, isRunning }: { executionId: number; isRunning: boolean }) {
  const scrollRef = useRef<HTMLDivElement>(null)

  const { data: events = [], dataUpdatedAt } = useQuery({
    queryKey: ['execution-events', executionId],
    queryFn: () => api.executions.events(executionId),
    refetchInterval: isRunning ? 3000 : false,
  })

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [dataUpdatedAt])

  if (events.length === 0) {
    return (
      <div className="flex items-center justify-center py-8 text-zinc-500 text-sm">
        {isRunning ? (
          <span className="flex items-center gap-2">
            <RefreshCw className="h-3 w-3 animate-spin" />
            Waiting for events...
          </span>
        ) : (
          'No events recorded'
        )}
      </div>
    )
  }

  return (
    <div
      ref={scrollRef}
      className="h-[400px] overflow-y-auto rounded-md border border-zinc-800 bg-zinc-950 p-2 space-y-0.5"
    >
      {events.map(event => (
        <div key={event.id} className="flex items-start gap-2 font-mono text-xs py-0.5">
          <span className="text-zinc-600 shrink-0 pt-0.5">
            {new Date(event.ts).toLocaleTimeString()}
          </span>
          <span className={cn('uppercase font-semibold shrink-0 w-10 pt-0.5', LEVEL_COLORS[event.level])}>
            {event.level}
          </span>
          <span className="text-zinc-500 shrink-0 pt-0.5">[{event.event_type}]</span>
          <span className="text-zinc-300 break-all">{event.message}</span>
        </div>
      ))}
    </div>
  )
}

function AgentRunList({ executionId }: { executionId: number }) {
  const { data: runs = [], isLoading } = useQuery({
    queryKey: ['execution-runs', executionId],
    queryFn: () => api.executions.runs(executionId),
    refetchInterval: 10000,
  })

  if (isLoading) {
    return <div className="py-4 text-center text-zinc-500 text-sm">Loading runs...</div>
  }

  if (runs.length === 0) {
    return <div className="py-4 text-center text-zinc-500 text-sm">No agent runs yet</div>
  }

  const outcomeColors: Record<string, string> = {
    success: 'bg-green-900/60 text-green-300',
    partial: 'bg-yellow-900/60 text-yellow-300',
    failed: 'bg-red-900/60 text-red-300',
  }

  return (
    <div className="space-y-3">
      {runs.map(run => (
        <Card key={run.id}>
          <CardContent className="p-4 space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-xs font-mono font-semibold text-zinc-400 uppercase">
                  {run.agent_type}
                </span>
                <span className="text-xs text-zinc-500">{run.role}</span>
              </div>
              <span
                className={cn(
                  'inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold',
                  outcomeColors[run.outcome] ?? 'bg-zinc-700 text-zinc-300'
                )}
              >
                {run.outcome}
              </span>
            </div>
            {run.summary && (
              <p className="text-sm text-zinc-300">{run.summary}</p>
            )}
            {run.files_changed && (
              <div className="text-xs text-zinc-500 font-mono">
                Files: {run.files_changed}
              </div>
            )}
            <div className="text-xs text-zinc-600">{formatDate(run.created_at)}</div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

export function ExecutionPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const executionId = Number(id)

  const { data: execution, isLoading, refetch } = useQuery({
    queryKey: ['execution', executionId],
    queryFn: () => api.executions.get(executionId),
    // Poll every 5s — we narrow down after first load using the isRunning/isPending flags
    refetchInterval: 5000,
  })

  const { data: task } = useQuery({
    queryKey: ['task', execution?.task_id],
    queryFn: () => api.tasks.get(execution!.task_id),
    enabled: !!execution?.task_id,
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-6 w-6 animate-spin text-zinc-500" />
      </div>
    )
  }

  if (!execution) {
    return (
      <div className="p-8 text-center">
        <p className="text-zinc-400 mb-4">Execution not found</p>
        <Button variant="outline" onClick={() => navigate('/dashboard')}>
          <ArrowLeft className="h-4 w-4" />
          Back to Dashboard
        </Button>
      </div>
    )
  }

  const isRunning = execution.status === 'running'
  const isDiffReview = execution.status === 'diff_review'
  const isFinished = ['completed', 'failed', 'interrupted'].includes(execution.status)
  const isPending = execution.status === 'pending' || execution.status === 'review'

  const statusIcon = () => {
    switch (execution.status) {
      case 'completed': return <CheckCircle className="h-5 w-5 text-green-400" />
      case 'failed': return <XCircle className="h-5 w-5 text-red-400" />
      case 'interrupted': return <AlertTriangle className="h-5 w-5 text-orange-400" />
      case 'running': return <RefreshCw className="h-5 w-5 text-blue-400 animate-spin" />
      default: return <Clock className="h-5 w-5 text-zinc-400" />
    }
  }

  return (
    <div className="p-6 space-y-6 max-w-5xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => navigate('/dashboard')}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex items-center gap-3 flex-1">
          {statusIcon()}
          <div>
            <h1 className="text-xl font-bold text-zinc-100">
              {task?.title ?? `Execution #${executionId}`}
            </h1>
            <p className="text-xs text-zinc-500">Execution #{executionId}</p>
          </div>
        </div>
        <span
          className={cn(
            'inline-flex items-center px-2.5 py-1 rounded text-xs font-semibold uppercase',
            STATUS_STYLES[execution.status]
          )}
        >
          {execution.status.replace('_', ' ')}
        </span>
        <Button variant="ghost" size="sm" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4" />
        </Button>
      </div>

      {/* Meta info */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div className="rounded-md border border-zinc-800 bg-zinc-900/50 p-3">
          <div className="text-xs text-zinc-500 mb-1">Base Branch</div>
          <div className="flex items-center gap-1 text-sm text-zinc-200 font-mono">
            <GitBranch className="h-3 w-3 text-zinc-500" />
            {execution.base_branch}
          </div>
        </div>
        <div className="rounded-md border border-zinc-800 bg-zinc-900/50 p-3">
          <div className="text-xs text-zinc-500 mb-1">Exec Branch</div>
          <div className="flex items-center gap-1 text-sm text-zinc-200 font-mono">
            <GitBranch className="h-3 w-3 text-zinc-500" />
            {execution.exec_branch}
          </div>
        </div>
        <div className="rounded-md border border-zinc-800 bg-zinc-900/50 p-3">
          <div className="text-xs text-zinc-500 mb-1">Started</div>
          <div className="text-sm text-zinc-200">
            {execution.started_at ? timeAgo(execution.started_at) : 'Not started'}
          </div>
        </div>
        <div className="rounded-md border border-zinc-800 bg-zinc-900/50 p-3">
          <div className="text-xs text-zinc-500 mb-1">Finished</div>
          <div className="text-sm text-zinc-200">
            {execution.finished_at ? timeAgo(execution.finished_at) : '—'}
          </div>
        </div>
      </div>

      {/* Task prompt if available */}
      {task && (
        <Card>
          <CardContent className="p-4">
            <div className="text-xs text-zinc-500 uppercase tracking-wider mb-2">Task Prompt</div>
            <p className="text-sm text-zinc-300 whitespace-pre-wrap">{task.prompt}</p>
          </CardContent>
        </Card>
      )}

      {/* Status-specific content */}
      {isPending && (
        <div className="flex flex-col items-center justify-center py-16 text-zinc-500">
          <Clock className="h-8 w-8 mb-3 text-zinc-600" />
          <p className="text-base">
            {execution.status === 'review'
              ? 'Commander is reviewing this task...'
              : 'Awaiting Commander review...'}
          </p>
          <p className="text-xs mt-2">This page will update automatically</p>
        </div>
      )}

      {(isRunning || isFinished) && (
        <Tabs defaultValue="events">
          <TabsList>
            <TabsTrigger value="events">Event Log</TabsTrigger>
            <TabsTrigger value="runs">Agent Runs</TabsTrigger>
          </TabsList>
          <TabsContent value="events" className="mt-4">
            <EventLog executionId={executionId} isRunning={isRunning} />
          </TabsContent>
          <TabsContent value="runs" className="mt-4">
            <AgentRunList executionId={executionId} />
          </TabsContent>
        </Tabs>
      )}

      {isDiffReview && (
        <div className="space-y-4">
          <div>
            <h2 className="text-lg font-semibold text-zinc-100 mb-1">Diff Review</h2>
            <p className="text-sm text-zinc-500">
              Review the changes made by BORE. Commit, merge, or revert them.
            </p>
          </div>
          <DiffView executionId={executionId} />
          <div className="mt-6">
            <Tabs defaultValue="runs">
              <TabsList>
                <TabsTrigger value="runs">Agent Runs</TabsTrigger>
                <TabsTrigger value="events">Event Log</TabsTrigger>
              </TabsList>
              <TabsContent value="runs" className="mt-4">
                <AgentRunList executionId={executionId} />
              </TabsContent>
              <TabsContent value="events" className="mt-4">
                <EventLog executionId={executionId} isRunning={false} />
              </TabsContent>
            </Tabs>
          </div>
        </div>
      )}
    </div>
  )
}
