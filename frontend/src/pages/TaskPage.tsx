import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft, Clock, RefreshCw, CheckCircle2, AlertTriangle,
  ChevronRight, Users, Layers, MessageSquare, SkipForward,
} from 'lucide-react'
import { api } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import type { ExecutionOption, OptionsResponse, ClarificationQuestion, ClarificationsResponse } from '@/lib/types'

export function TaskPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const taskId = Number(id)

  const [selectedOption, setSelectedOption] = useState<string | null>(null)
  const [answers, setAnswers] = useState<Record<string, string>>({})

  const { data: task, isLoading: taskLoading } = useQuery({
    queryKey: ['task', taskId],
    queryFn: () => api.tasks.get(taskId),
    refetchInterval: 3000,
  })

  const isActiveTask = task
    ? ['pending', 'review', 'running', 'diff_review'].includes(task.status)
    : true

  const { data: reviews = [] } = useQuery({
    queryKey: ['task-reviews', taskId],
    queryFn: () => api.tasks.reviews(taskId),
    refetchInterval: isActiveTask ? 2000 : false,
  })

  const { data: executions = [] } = useQuery({
    queryKey: ['executions'],
    queryFn: api.executions.list,
    refetchInterval: isActiveTask ? 3000 : false,
  })

  const exec = executions.find(e => e.task_id === taskId)

  // Navigate to execution page once execution exists and task is running/diff_review/completed
  useEffect(() => {
    if (exec && task && ['running', 'diff_review', 'completed', 'failed'].includes(task.status)) {
      navigate(`/executions/${exec.id}`, { replace: true })
    }
  }, [exec, task, navigate])

  // --- Parse reviews ---
  const clarificationReview = reviews.find(r => r.phase === 'clarification')
  const hasClarificationAnswers = reviews.some(r => r.phase === 'clarification_answers')
  const optionsReview = reviews.find(r => r.phase === 'options')
  const hasSelection = reviews.some(r => r.phase === 'selection')

  let clarificationQuestions: ClarificationQuestion[] = []
  if (clarificationReview) {
    try {
      const parsed: ClarificationsResponse = JSON.parse(clarificationReview.content)
      clarificationQuestions = parsed.questions || []
    } catch {
      // ignore malformed JSON
    }
  }

  let options: ExecutionOption[] = []
  if (optionsReview) {
    try {
      const parsed: OptionsResponse = JSON.parse(optionsReview.content)
      options = parsed.options || []
    } catch {
      try {
        options = JSON.parse(optionsReview.content)
      } catch {
        // give up
      }
    }
  }

  // --- Mutations ---
  const submitClarificationsMutation = useMutation({
    mutationFn: (ans: Record<string, string>) => api.tasks.submitClarifications(taskId, ans),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['task-reviews', taskId] })
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
    },
  })

  const submitOptionMutation = useMutation({
    mutationFn: (optionId: string) => api.tasks.submitReview(taskId, optionId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['task', taskId] })
      queryClient.invalidateQueries({ queryKey: ['task-reviews', taskId] })
      queryClient.invalidateQueries({ queryKey: ['tasks'] })
      setSelectedOption(null)
    },
  })

  // --- Derived UI state flags ---
  const isPendingFresh = task?.status === 'pending' && !clarificationReview && !hasClarificationAnswers && !hasSelection
  const isPendingGeneratingOptions = task?.status === 'pending' && hasClarificationAnswers && !hasSelection
  const isPendingBrief = task?.status === 'pending' && hasSelection
  const isAnalyzing = task?.status === 'review' && !clarificationReview
  const showClarifications = task?.status === 'review'
    && clarificationQuestions.length > 0
    && !hasClarificationAnswers
    && !optionsReview
  const isGeneratingOptions = task?.status === 'review'
    && clarificationReview != null
    && !optionsReview
    && (clarificationQuestions.length === 0 || hasClarificationAnswers)
  const showOptions = task?.status === 'review' && options.length > 0 && !hasSelection

  if (taskLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <RefreshCw className="h-6 w-6 animate-spin text-zinc-500" />
      </div>
    )
  }

  if (!task) {
    return (
      <div className="p-8 text-center">
        <p className="text-zinc-400 mb-4">Task not found</p>
        <Button variant="outline" onClick={() => navigate('/dashboard')}>
          <ArrowLeft className="h-4 w-4" />
          Back to Dashboard
        </Button>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6 max-w-5xl mx-auto">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => navigate('/dashboard')}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex items-center gap-3">
          <Clock className="h-5 w-5 text-zinc-400" />
          <h1 className="text-xl font-bold text-zinc-100">{task.title}</h1>
        </div>
        <StatusBadge status={task.status} />
      </div>

      {/* Task Prompt */}
      <Card>
        <CardContent className="p-4">
          <div className="text-xs text-zinc-500 uppercase tracking-wider mb-2">Task Prompt</div>
          <p className="text-sm text-zinc-300 whitespace-pre-wrap">{task.prompt}</p>
        </CardContent>
      </Card>

      {/* === PLANNING PHASE STATES === */}

      {/* 1. Fresh task, not yet picked up */}
      {isPendingFresh && (
        <Spinner label="Waiting for Commander to pick up this task..." sub="This page will update automatically" />
      )}

      {/* 2. Picked up, running clarifications (status just flipped to review) */}
      {isAnalyzing && (
        <Spinner label="Commander is analyzing your task..." sub="Clarification questions will appear shortly" />
      )}

      {/* 3. Clarification questions — interactive */}
      {showClarifications && (
        <div className="space-y-5">
          <div className="flex items-center gap-2">
            <MessageSquare className="h-5 w-5 text-amber-400" />
            <h2 className="text-lg font-semibold text-zinc-100">Commander has questions</h2>
          </div>
          <p className="text-sm text-zinc-400">
            Answer these questions to help the Commander propose the best approach.
            You can skip any question you don't want to answer.
          </p>
          <div className="space-y-4">
            {clarificationQuestions.map((q) => (
              <Card key={q.id} className="border-zinc-700 bg-zinc-900">
                <CardContent className="p-4 space-y-3">
                  <div>
                    <p className="text-sm font-medium text-zinc-100">{q.question}</p>
                    {q.why && (
                      <p className="text-xs text-zinc-500 mt-1 italic">{q.why}</p>
                    )}
                  </div>
                  <Textarea
                    placeholder="Your answer (leave blank to skip)..."
                    value={answers[q.id] ?? ''}
                    onChange={e => setAnswers(prev => ({ ...prev, [q.id]: e.target.value }))}
                    className="bg-zinc-800 border-zinc-700 text-zinc-100 placeholder:text-zinc-600 resize-none"
                    rows={2}
                  />
                </CardContent>
              </Card>
            ))}
          </div>
          <div className="flex items-center gap-3 pt-1">
            <Button
              onClick={() => submitClarificationsMutation.mutate(answers)}
              disabled={submitClarificationsMutation.isPending}
              className="bg-amber-600 hover:bg-amber-700 text-white"
            >
              {submitClarificationsMutation.isPending ? (
                <>
                  <RefreshCw className="h-4 w-4 animate-spin mr-2" />
                  Submitting...
                </>
              ) : (
                <>
                  <CheckCircle2 className="h-4 w-4 mr-2" />
                  Submit Answers
                </>
              )}
            </Button>
            <Button
              variant="ghost"
              onClick={() => submitClarificationsMutation.mutate({})}
              disabled={submitClarificationsMutation.isPending}
              className="text-zinc-400 hover:text-zinc-200"
            >
              <SkipForward className="h-4 w-4 mr-2" />
              Skip — just generate options
            </Button>
          </div>
        </div>
      )}

      {/* 4. Waiting for task runner to pick up answers and generate options */}
      {isPendingGeneratingOptions && (
        <Spinner label="Generating execution options..." sub="Commander is proposing approaches based on your answers" />
      )}

      {/* 5. Options being generated (status=review, no options yet, clarifications done) */}
      {isGeneratingOptions && (
        <Spinner label="Commander is generating options..." sub="Execution approaches will appear here shortly" />
      )}

      {/* 6. Options ready — pick one */}
      {showOptions && (
        <div className="space-y-4">
          <div className="flex items-center gap-2">
            <Layers className="h-5 w-5 text-blue-400" />
            <h2 className="text-lg font-semibold text-zinc-100">Commander Options</h2>
          </div>
          <p className="text-sm text-zinc-400">
            Pick an approach to proceed. The selected option will be used to generate an execution brief for the workers.
          </p>
          <div className="grid gap-4">
            {options.map((opt) => (
              <OptionCard
                key={opt.id}
                option={opt}
                selected={selectedOption === opt.id}
                onSelect={() => setSelectedOption(opt.id)}
              />
            ))}
          </div>
          {selectedOption && (
            <div className="flex items-center gap-3 pt-2">
              <Button
                onClick={() => submitOptionMutation.mutate(selectedOption)}
                disabled={submitOptionMutation.isPending}
                className="bg-blue-600 hover:bg-blue-700 text-white"
              >
                {submitOptionMutation.isPending ? (
                  <>
                    <RefreshCw className="h-4 w-4 animate-spin mr-2" />
                    Submitting...
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="h-4 w-4 mr-2" />
                    Execute Option {selectedOption.toUpperCase()}
                  </>
                )}
              </Button>
              <Button variant="ghost" onClick={() => setSelectedOption(null)}>
                Cancel
              </Button>
            </div>
          )}
        </div>
      )}

      {/* 7. Option selected — generating execution brief */}
      {isPendingBrief && (
        <Spinner label="Generating execution brief..." sub="Commander is preparing instructions for the Boss" />
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function Spinner({ label, sub }: { label: string; sub?: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-zinc-500">
      <RefreshCw className="h-8 w-8 mb-3 text-zinc-600 animate-spin" />
      <p className="text-base text-zinc-300">{label}</p>
      {sub && <p className="text-xs mt-2">{sub}</p>}
    </div>
  )
}

function OptionCard({ option, selected, onSelect }: {
  option: ExecutionOption
  selected: boolean
  onSelect: () => void
}) {
  return (
    <Card
      className={`cursor-pointer transition-all ${
        selected
          ? 'border-blue-500 bg-blue-500/10 ring-1 ring-blue-500/50'
          : 'border-zinc-700 hover:border-zinc-500 bg-zinc-900'
      }`}
      onClick={onSelect}
    >
      <CardContent className="p-5">
        <div className="flex items-start gap-4">
          {/* Selection indicator */}
          <div className={`mt-1 w-5 h-5 rounded-full border-2 flex items-center justify-center shrink-0 ${
            selected ? 'border-blue-500 bg-blue-500' : 'border-zinc-600'
          }`}>
            {selected && <CheckCircle2 className="h-3 w-3 text-white" />}
          </div>

          <div className="flex-1 space-y-3">
            {/* Title + ID */}
            <div className="flex items-center gap-2">
              <span className="text-xs font-mono px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700">
                {option.id.toUpperCase()}
              </span>
              <h3 className="text-base font-semibold text-zinc-100">{option.title}</h3>
            </div>

            {/* Summary */}
            <p className="text-sm text-zinc-300">{option.summary}</p>

            {/* Approach steps */}
            {option.approach_steps && option.approach_steps.length > 0 && (
              <div>
                <div className="text-xs text-zinc-500 uppercase tracking-wider mb-1">Approach</div>
                <ul className="space-y-1">
                  {option.approach_steps.map((step, i) => (
                    <li key={i} className="text-sm text-zinc-400 flex items-start gap-2">
                      <ChevronRight className="h-3 w-3 mt-1 shrink-0 text-zinc-600" />
                      {step}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {/* Meta row */}
            <div className="flex flex-wrap gap-4 text-xs text-zinc-500">
              {option.crew_suggestion && option.crew_suggestion !== 'none' && (
                <span className="flex items-center gap-1">
                  <Users className="h-3 w-3" />
                  Crew: {option.crew_suggestion}
                </span>
              )}
              {option.worker_budget_suggestion > 0 && (
                <span className="flex items-center gap-1">
                  <Layers className="h-3 w-3" />
                  {option.worker_budget_suggestion} worker{option.worker_budget_suggestion !== 1 ? 's' : ''}
                </span>
              )}
            </div>

            {/* Risks */}
            {option.risks && option.risks.length > 0 && (
              <div>
                <div className="text-xs text-zinc-500 uppercase tracking-wider mb-1">Risks</div>
                <div className="flex flex-wrap gap-2">
                  {option.risks.map((risk, i) => (
                    <span key={i} className="text-xs px-2 py-1 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 flex items-center gap-1">
                      <AlertTriangle className="h-3 w-3" />
                      {risk}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: 'bg-zinc-700 text-zinc-300',
    review: 'bg-blue-500/20 text-blue-400 border border-blue-500/30',
    running: 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30',
    diff_review: 'bg-purple-500/20 text-purple-400 border border-purple-500/30',
    completed: 'bg-green-500/20 text-green-400 border border-green-500/30',
    failed: 'bg-red-500/20 text-red-400 border border-red-500/30',
    interrupted: 'bg-orange-500/20 text-orange-400 border border-orange-500/30',
  }
  return (
    <span className={`text-xs px-2 py-1 rounded font-medium uppercase tracking-wider ${colors[status] || 'bg-zinc-700 text-zinc-300'}`}>
      {status.replace('_', ' ')}
    </span>
  )
}
