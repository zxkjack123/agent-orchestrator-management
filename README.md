# Agents-Orchestfator-Management

This repository now contains both planning documents and the current implementation foundation for AOM.

Key documents:
- Current status and handoff: [docs/current-status.md](docs/current-status.md)
- Experimental real-agent E2E plan: [docs/experimental-agent-e2e-plan.md](docs/experimental-agent-e2e-plan.md)
- Main planning document: [docs/AOM-planning.md](docs/AOM-planning.md)
- Milestone plan: [docs/AOM-milestones.md](docs/AOM-milestones.md)
- Milestone 1 implementation plan: [docs/milestone-1-implementation-plan.md](docs/milestone-1-implementation-plan.md)
- Milestone 2 implementation plan: [docs/milestone-2-implementation-plan.md](docs/milestone-2-implementation-plan.md)
- Project structure: [docs/project-structure.md](docs/project-structure.md)
- Engineering guidelines: [docs/engineering-guidelines.md](docs/engineering-guidelines.md)

AOM is intended to be a project-level control plane for specialist CLI agents, focused on:

- state continuity
- session lifecycle
- git worktree isolation
- project-scoped resources
- workflow handoff

Current progress:
- Milestone 0: completed
- Milestone 1: completed
- Milestone 2: completed in code, tests, and live local E2E
- Milestone 3: implemented core task and step workflow, plan generation, and `plan --create`
- Milestone 4: in progress with canonical task artifacts, task-bound lifecycle logging, and artifact-path reporting
- Milestone 5: in progress with task worktree continuity, repair, replacement, and one-writer-per-worktree guardrails
- Milestone 6: first slice implemented with `checkpoint` and `handoff`
