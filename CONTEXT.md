# Recall Context

## What Recall is

Recall is a local-first context recorder for agent-assisted development.

It helps developers keep ownership of their code while using coding agents heavily.

## Problem

When using agents, code can change faster than the human can maintain context.

This leads to questions like:

- What changed?
- Why did we do this?
- Which files matter?
- What was the plan?
- What should I do next?
- Do I still understand this codebase?

## Personal pain this solves

The core issue discovered was not lack of programming ability. It was context and direction.

Patterns:

- Projects are started but often not finished.
- Choosing ideas, setup, and design are common blockers.
- Ideas currently live mostly in the user's head.
- Repos often get abandoned and not revisited.
- The user can grind when the goal is clear.
- Boredom causes project switching.
- Heavy agent usage causes context loss.
- Projects become boring when they are vibe-coded too much and the user is no longer fully context-aware.

## Product thesis

Coding agents are getting more capable, but developers need tools to preserve human context, project memory, and decision history.

Recall should help answer:

- What was I trying to do?
- What changed since I started?
- What has not been reviewed?
- What files are important?
- What decisions were made?
- What is risky?
- What should I do next?
- What context should I give the next agent?

## Initial CLI idea

```bash
recall init
recall start "build auth flow"
recall status
recall checkpoint "OAuth flow implemented"
recall review
recall handoff
recall stop
```

## MVP scope

The first version should be deterministic and local. No AI dependency required.

Features:

- Create `.recall/` in a project
- Track the active session goal
- Inspect Git state
- Summarize changed files and diff stats
- Create Markdown checkpoints
- Generate an agent handoff file
- Produce a review checklist when context-loss risk is high

## Later ideas

- TUI timeline
- Shell command capture
- Agent adapters for Pi, Claude Code, Cursor, Aider, Codex, OpenCode
- Neovim plugin
- Session replay
- Optional AI summaries

## Positioning

> Recall helps developers stay in control while coding with AI agents.

Alternative:

> Recall creates checkpoints, handoffs, and review prompts so agent-written code does not become mystery code.

## Philosophy

Recall is not another coding agent.

It is a tool for the human using agents.

The goal is not to automate more. The goal is to preserve context, momentum, and ownership.
