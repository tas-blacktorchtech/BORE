import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw, Users, ChevronDown, ChevronRight, Trash2, Edit2 } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { cn, timeAgo } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import type {
  Task,
  Crew,
  TaskStatus,
  Complexity,
  Mode,
  CreateTaskInput,
  CreateThreadInput,
} from '@/lib/types'

const STATUS_STYLES: Record<TaskStatus, string> = {
  pending: 'bg-zinc-700 text-zinc-300',
  review: 'bg-yellow-900/60 text-yellow-300',
  running: 'bg-blue-900/60 text-blue-300 animate-pulse',
  diff_review: 'bg-purple-900/60 text-purple-300',
  completed: 'bg-green-900/60 text-green-300',
  failed: 'bg-red-900/60 text-red-300',
  interrupted: 'bg-orange-900/60 text-orange-300',
}

const STATUS_LABELS: Record<TaskStatus, string> = {
  pending: 'PEND',
  review: 'REVIEW',
  running: 'RUN',
  diff_review: 'DIFF',
  completed: 'DONE',
  failed: 'FAIL',
  interrupted: 'INT',
}

interface CrewFormState {
  name: string
  objective: string
  constraints: string
  allowed_commands: string
  ownership_paths: string
}

const defaultCrewForm = (): CrewFormState => ({
  name: '',
  objective: '',
  constraints: '',
  allowed_commands: '',
  ownership_paths: '',
})

export function Dashboard() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const queryClient = useQueryClient()
  const threadFilter = searchParams.get('thread')

  const [showNewTask, setShowNewTask] = useState(false)
  const [showCrews, setShowCrews] = useState(false)
  const [editingCrew, setEditingCrew] = useState<Crew | null>(null)
  const [showNewCrew, setShowNewCrew] = useState(false)

  // Task form state
  const [taskTitle, setTaskTitle] = useState('')
  const [taskPrompt, setTaskPrompt] = useState('')
  const [taskComplexity, setTaskComplexity] = useState<Complexity>('medium')
  const [taskMode, setTaskMode] = useState<Mode>('just_get_it_done')
  const [taskThreadId, setTaskThreadId] = useState<number | null>(null)
  const [threadSelection, setThreadSelection] = useState('none')
  const [newThreadName, setNewThreadName] = useState('')
  const [newThreadDescription, setNewThreadDescription] = useState('')

  // Crew form state
  const [crewForm, setCrewForm] = useState<CrewFormState>(defaultCrewForm())

  const { data: tasks = [], refetch: refetchTasks } = useQuery({
    queryKey: ['tasks'],
    queryFn: api.tasks.list,
    refetchInterval: 3000,
  })

  const { data: executions = [] } = useQuery({
    queryKey: ['executions'],
    queryFn: api.executions.list,
    refetchInterval: 3000,
  })

  const { data: threads = [] } = useQuery({
    queryKey: ['threads'],
    queryFn: api.threads.list,
  })

  const { data: crews = [], refetch: refetchCrews } = useQuery({
    queryKey: ['crews'],
    queryFn: api.crews.list,
  })

  const createTaskMutation = useMutation({
    mutationFn: (input: CreateTaskInput) => api.tasks.create(input),
    onSuccess: (task) => {
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
      queryClient.invalidateQueries({ queryKey: ['executions'] })
      navigate(`/tasks/${task.id}`)
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  })

  const createThreadMutation = useMutation({
    mutationFn: (input: CreateThreadInput) => api.threads.create(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['threads'] })
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  })

  const deleteCrewMutation = useMutation({
    mutationFn: (id: number) => api.crews.delete(id),
    onSuccess: () => {
      toast.success('Crew deleted')
      refetchCrews()
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  })

  const createCrewMutation = useMutation({
    mutationFn: (form: CrewFormState) => api.crews.create(form),
    onSuccess: () => {
      toast.success('Crew created')
      setShowNewCrew(false)
      setCrewForm(defaultCrewForm())
      refetchCrews()
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  })

  const updateCrewMutation = useMutation({
    mutationFn: ({ id, form }: { id: number; form: CrewFormState }) =>
      api.crews.update(id, form),
    onSuccess: () => {
      toast.success('Crew updated')
      setEditingCrew(null)
      refetchCrews()
    },
    onError: (e: Error) => toast.error(`Failed: ${e.message}`),
  })

  // Stats
  const running = tasks.filter(t => t.status === 'running').length
  const completed = tasks.filter(t => t.status === 'completed').length
  const failed = tasks.filter(t => t.status === 'failed' || t.status === 'interrupted').length

  // Filter by thread
  const filteredTasks = threadFilter
    ? tasks.filter(t => String(t.thread_id) === threadFilter)
    : tasks

  const getExecutionForTask = (taskId: number) =>
    executions.find(e => e.task_id === taskId)

  const handleTaskClick = (task: Task) => {
    const exec = getExecutionForTask(task.id)
    if (exec) {
      navigate(`/executions/${exec.id}`)
    } else {
      navigate(`/tasks/${task.id}`)
    }
  }

  const openEditCrew = (crew: Crew) => {
    setEditingCrew(crew)
    setCrewForm({
      name: crew.name,
      objective: crew.objective,
      constraints: crew.constraints,
      allowed_commands: crew.allowed_commands,
      ownership_paths: crew.ownership_paths,
    })
  }

  const handleThreadSelection = (value: string) => {
    setThreadSelection(value)
    if (value === 'none' || value === 'new') {
      setTaskThreadId(null)
      return
    }
    setTaskThreadId(Number(value))
  }

  const handleCreateTask = async () => {
    let threadId = taskThreadId
    if (threadSelection === 'new') {
      const name = newThreadName.trim()
      if (!name) {
        toast.error('Thread name is required')
        return
      }
      try {
        const thread = await createThreadMutation.mutateAsync({
          name,
          description: newThreadDescription.trim(),
        })
        threadId = thread.id
      } catch (error) {
        return
      }
    }
    createTaskMutation.mutate({
      title: taskTitle,
      prompt: taskPrompt,
      complexity: taskComplexity,
      mode: taskMode,
      thread_id: threadId,
    })
  }

  const createTaskDisabled =
    !taskTitle ||
    !taskPrompt ||
    createTaskMutation.isPending ||
    createThreadMutation.isPending ||
    (threadSelection === 'new' && !newThreadName.trim())

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-zinc-100">Dashboard</h1>
        <div className="flex gap-2">
          <Button variant="ghost" size="sm" onClick={() => refetchTasks()}>
            <RefreshCw className="h-4 w-4" />
          </Button>
          <Button onClick={() => setShowNewTask(true)}>
            <Plus className="h-4 w-4" />
            New Task
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4">
        {[
          { label: 'Total', value: tasks.length, color: 'text-zinc-100' },
          { label: 'Running', value: running, color: 'text-blue-400' },
          { label: 'Completed', value: completed, color: 'text-green-400' },
          { label: 'Failed', value: failed, color: 'text-red-400' },
        ].map(s => (
          <Card key={s.label}>
            <CardContent className="p-4">
              <div className={cn('text-2xl font-bold', s.color)}>{s.value}</div>
              <div className="text-xs text-zinc-500 mt-1">{s.label}</div>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Thread filter */}
      {threads.length > 0 && (
        <div className="flex gap-2 flex-wrap">
          <button
            onClick={() => setSearchParams({})}
            className={cn(
              'px-3 py-1 text-xs rounded-full border transition-colors',
              !threadFilter
                ? 'border-zinc-500 bg-zinc-800 text-zinc-100'
                : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
            )}
          >
            All ({tasks.length})
          </button>
          {threads.map(t => (
            <button
              key={t.id}
              onClick={() => setSearchParams({ thread: String(t.id) })}
              className={cn(
                'px-3 py-1 text-xs rounded-full border transition-colors',
                threadFilter === String(t.id)
                  ? 'border-zinc-500 bg-zinc-800 text-zinc-100'
                  : 'border-zinc-700 text-zinc-400 hover:border-zinc-500'
              )}
            >
              {t.name} ({tasks.filter(tk => tk.thread_id === t.id).length})
            </button>
          ))}
        </div>
      )}

      {/* Task list */}
      <div className="space-y-2">
        {filteredTasks.length === 0 ? (
          <div className="text-center py-16 text-zinc-500">
            <p className="mb-4">No tasks yet</p>
            <Button onClick={() => setShowNewTask(true)}>
              <Plus className="h-4 w-4" />
              Create your first task
            </Button>
          </div>
        ) : (
          filteredTasks
            .slice()
            .reverse()
            .map(task => (
              <Card
                key={task.id}
                className={cn(
                  'cursor-pointer hover:border-zinc-600 transition-colors',
                  task.status === 'running' && 'border-blue-800/50'
                )}
                onClick={() => handleTaskClick(task)}
              >
                <CardContent className="flex items-center justify-between p-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3">
                      <span
                        className={cn(
                          'inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-semibold',
                          STATUS_STYLES[task.status]
                        )}
                      >
                        {STATUS_LABELS[task.status]}
                      </span>
                      <span className="font-medium text-zinc-100 truncate">{task.title}</span>
                    </div>
                    <div className="flex items-center gap-3 mt-1">
                      <span className="text-xs text-zinc-500">{timeAgo(task.created_at)}</span>
                      <span className="text-xs text-zinc-600">·</span>
                      <span className="text-xs text-zinc-500">{task.complexity}</span>
                      {threads.find(t => t.id === task.thread_id) && (
                        <>
                          <span className="text-xs text-zinc-600">·</span>
                          <span className="text-xs text-zinc-500">
                            {threads.find(t => t.id === task.thread_id)?.name}
                          </span>
                        </>
                      )}
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))
        )}
      </div>

      {/* Crews section (collapsible) */}
      <div>
        <button
          onClick={() => setShowCrews(c => !c)}
          className="flex items-center gap-2 text-sm font-medium text-zinc-400 hover:text-zinc-100 transition-colors mb-3"
        >
          {showCrews ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
          <Users className="h-4 w-4" />
          Crews ({crews.length})
        </button>
        {showCrews && (
          <div className="space-y-2">
            {crews.length === 0 && (
              <p className="text-sm text-zinc-500 py-2">No crews configured yet.</p>
            )}
            {crews.map(crew => (
              <Card key={crew.id}>
                <CardContent className="flex items-center justify-between p-4">
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-zinc-100">{crew.name}</div>
                    <div className="text-xs text-zinc-500 mt-1 truncate">{crew.objective}</div>
                  </div>
                  <div className="flex gap-2 ml-4">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => openEditCrew(crew)}
                    >
                      <Edit2 className="h-3 w-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => deleteCrewMutation.mutate(crew.id)}
                      disabled={deleteCrewMutation.isPending}
                    >
                      <Trash2 className="h-3 w-3 text-red-400" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setCrewForm(defaultCrewForm())
                setShowNewCrew(true)
              }}
            >
              <Plus className="h-4 w-4" />
              Add Crew
            </Button>
          </div>
        )}
      </div>

      {/* New Task Dialog */}
      <Dialog open={showNewTask} onOpenChange={setShowNewTask}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>New Task</DialogTitle>
            <DialogDescription>Describe what you want BORE to do</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Title</Label>
              <Input
                value={taskTitle}
                onChange={e => setTaskTitle(e.target.value)}
                placeholder="Task title"
                className="mt-1"
              />
            </div>
            <div>
              <Label>Prompt</Label>
              <Textarea
                value={taskPrompt}
                onChange={e => setTaskPrompt(e.target.value)}
                placeholder="Describe the task in detail..."
                className="mt-1 min-h-30"
              />
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <Label>Complexity</Label>
                <Select
                  value={taskComplexity}
                  onValueChange={v => setTaskComplexity(v as Complexity)}
                >
                  <SelectTrigger className="mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="basic">Basic</SelectItem>
                    <SelectItem value="medium">Medium</SelectItem>
                    <SelectItem value="complex">Complex</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>Mode</Label>
                <Select value={taskMode} onValueChange={v => setTaskMode(v as Mode)}>
                  <SelectTrigger className="mt-1">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="just_get_it_done">Just Do It</SelectItem>
                    <SelectItem value="alert_with_issues">Alert on Issues</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div>
              <Label>Thread (optional)</Label>
              <Select value={threadSelection} onValueChange={handleThreadSelection}>
                <SelectTrigger className="mt-1">
                  <SelectValue placeholder="No thread" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">No thread</SelectItem>
                  <SelectItem value="new">Create new thread...</SelectItem>
                  {threads.map(t => (
                    <SelectItem key={t.id} value={String(t.id)}>
                      {t.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {threadSelection === 'new' && (
                <div className="mt-3 space-y-3">
                  <div>
                    <Label>Thread name</Label>
                    <Input
                      value={newThreadName}
                      onChange={e => setNewThreadName(e.target.value)}
                      placeholder="frontend-polish"
                      className="mt-1"
                    />
                  </div>
                  <div>
                    <Label>Description (optional)</Label>
                    <Textarea
                      value={newThreadDescription}
                      onChange={e => setNewThreadDescription(e.target.value)}
                      placeholder="What this thread covers"
                      className="mt-1"
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowNewTask(false)}>
              Cancel
            </Button>
            <Button onClick={handleCreateTask} disabled={createTaskDisabled}>
              {createTaskMutation.isPending || createThreadMutation.isPending
                ? 'Creating...'
                : 'Create Task'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* New Crew Dialog */}
      <Dialog open={showNewCrew} onOpenChange={setShowNewCrew}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>New Crew</DialogTitle>
            <DialogDescription>Configure a specialized agent crew</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Name</Label>
              <Input
                value={crewForm.name}
                onChange={e => setCrewForm(f => ({ ...f, name: e.target.value }))}
                placeholder="frontend-crew"
                className="mt-1"
              />
            </div>
            <div>
              <Label>Objective</Label>
              <Textarea
                value={crewForm.objective}
                onChange={e => setCrewForm(f => ({ ...f, objective: e.target.value }))}
                placeholder="What this crew specializes in..."
                className="mt-1"
              />
            </div>
            <div>
              <Label>Constraints</Label>
              <Textarea
                value={crewForm.constraints}
                onChange={e => setCrewForm(f => ({ ...f, constraints: e.target.value }))}
                placeholder="Optional constraints..."
                className="mt-1"
              />
            </div>
            <div>
              <Label>Allowed Commands</Label>
              <Input
                value={crewForm.allowed_commands}
                onChange={e => setCrewForm(f => ({ ...f, allowed_commands: e.target.value }))}
                placeholder="npm, yarn, git, ..."
                className="mt-1"
              />
            </div>
            <div>
              <Label>Ownership Paths</Label>
              <Input
                value={crewForm.ownership_paths}
                onChange={e => setCrewForm(f => ({ ...f, ownership_paths: e.target.value }))}
                placeholder="src/frontend/, ..."
                className="mt-1"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowNewCrew(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => createCrewMutation.mutate(crewForm)}
              disabled={!crewForm.name || createCrewMutation.isPending}
            >
              {createCrewMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Crew Dialog */}
      <Dialog open={!!editingCrew} onOpenChange={open => { if (!open) { setEditingCrew(null); setCrewForm(defaultCrewForm()) } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Crew</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>Name</Label>
              <Input
                value={crewForm.name}
                onChange={e => setCrewForm(f => ({ ...f, name: e.target.value }))}
                className="mt-1"
              />
            </div>
            <div>
              <Label>Objective</Label>
              <Textarea
                value={crewForm.objective}
                onChange={e => setCrewForm(f => ({ ...f, objective: e.target.value }))}
                className="mt-1"
              />
            </div>
            <div>
              <Label>Constraints</Label>
              <Textarea
                value={crewForm.constraints}
                onChange={e => setCrewForm(f => ({ ...f, constraints: e.target.value }))}
                className="mt-1"
              />
            </div>
            <div>
              <Label>Allowed Commands</Label>
              <Input
                value={crewForm.allowed_commands}
                onChange={e => setCrewForm(f => ({ ...f, allowed_commands: e.target.value }))}
                className="mt-1"
              />
            </div>
            <div>
              <Label>Ownership Paths</Label>
              <Input
                value={crewForm.ownership_paths}
                onChange={e => setCrewForm(f => ({ ...f, ownership_paths: e.target.value }))}
                className="mt-1"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setEditingCrew(null); setCrewForm(defaultCrewForm()) }}>
              Cancel
            </Button>
            <Button
              onClick={() => {
                if (editingCrew) {
                  updateCrewMutation.mutate({ id: editingCrew.id, form: crewForm })
                }
              }}
              disabled={!crewForm.name || updateCrewMutation.isPending}
            >
              {updateCrewMutation.isPending ? 'Saving...' : 'Save'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
