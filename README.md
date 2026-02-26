# BORE
**BlackTorch Orchestration Runtime Engine**

Local AI command center for software development.  
Run your project like an organization — **Commander → Teams → Workers** — with full control, visibility, and Git-native execution.

---

## What is BORE?

BORE is a **local-first AI orchestration system** that turns your repository into a structured AI workforce.

Instead of asking a single assistant to “write code”, BORE lets you:

- Define a **Commander** that plans work
- Organize **Teams** (UI, Backend, Infra, etc.)
- Give each team a **Boss** that manages execution
- Run multiple **Workers** and **QA agents**
- Execute changes in isolated **Git worktrees**
- Monitor everything in a live **operations dashboard**

This is not chat.  
This is **AI operations**.

---

## Core Concepts

### Cluster
A project workspace backed by a Git repository.


---

### Commander
- Receives tasks
- Generates a plan
- Suggests the appropriate team
- Requires **user approval** before execution

---

### Boss
- Executes the approved plan
- Spawns workers
- Runs QA / review steps
- Reports progress, timing, and issues

---

### Workers
- Implement changes
- Produce patches/diffs
- Run within isolated Git worktrees

---

### Auxiliary
- Code review
- Testing
- Validation
- No direct code changes by default

---

## Why BORE?

Most AI coding tools today are:

- Stateless
- Single-agent
- Opaque
- Risky for real projects

BORE provides:

- **Human-in-the-loop control**
- **Parallel task execution**
- **Organizational structure**
- **Persistent AI context**
- **Full Git safety**
- **Live operational visibility**

You don’t “ask AI for code”.

You **run an AI organization**.

---

## Key Features

- Local-first (no cloud required)
- Works with existing Git repositories
- Git worktree isolation per task
- Plan → Review → Approve → Execute workflow
- Parallel executions
- Streaming operational console
- Versioned AI memory inside `.bore/`
- Modern web UI (React + shadcn)
- Provider-based architecture (Claude CLI v1)

---

## Git Safety Model

Each execution runs in isolation:
.bore/worktrees/<execution-id>/
branch: bore/<execution-id>



BORE will **never**:

- Modify `main` automatically
- Auto-merge changes
- Delete work without user approval

After execution, you choose:

- Review diff
- Commit
- Merge
- Keep worktree
- Delete worktree

---

## Project Memory

Each repository contains:
.bore/
cluster.md
commander.md
teams/
executions/



This directory is the project’s **organizational memory** and should be committed to Git.

---

## Tech Stack

**Backend**
- Go
- SQLite
- Git CLI
- Claude CLI

**Frontend**
- React
- Vite
- TypeScript
- Tailwind
- shadcn/ui
- React Flow

---

## Current Status

BORE is in active development.

Planned for v1:

- Cluster creation (clone or attach repo)
- Commander + Team + Boss configuration
- Task planning with approval workflow
- Multi-worker execution
- Git worktree isolation
- Live execution dashboard
- Execution history written to `.bore/`

---

## Philosophy

BORE is:

- Local
- Deterministic
- Transparent
- Human-controlled
- Git-native

BORE is **not**:

- A chatbot
- Autonomous coding
- A cloud service
- Multi-user collaboration
- CI/CD replacement

---

## Example Workflow

1. Create a Cluster from your repo  
2. Define your Commander  
3. Create Teams (UI, Backend, etc.)  
4. Submit a task  

Task
→ Commander Plan
→ Edit / Approve
→ Boss Execution
→ Workers + QA
→ Review
→ Done



All changes land in a worktree.  
You decide what happens next.

---

## Vision

Software development is shifting from writing code  
to **directing AI systems**.

BORE is built for that future.

---

## License
MIT
