# PentAGI design ideas adapted into CTF-Agent

This note records the first practical adaptation pass after reviewing the local `/tmp/pentagi` clone.

## Design mapping

| PentAGI concept | CTF-Agent mapping | Implemented in this pass |
| --- | --- | --- |
| Flow | Solve session (`sessions`) | Existing session remains the top-level flow |
| Task/Subtask | Persisted per-agent execution units | New `subtasks` table and `/api/sessions/{id}/subtasks` |
| Action/ToolCall | Persisted tool executions | New `tool_calls` table and `/api/sessions/{id}/tool-calls` |
| Flow templates | Reusable playbooks | New `flow_templates` table and `/api/templates` CRUD |
| Provider/agent supervision | Agent recorder hook | New `agent.ExecutionRecorder` contract wired into PrimaryAgent, BaseAgent, and Orchestrator |
| Reporter/replay evidence | Timeline-ready data | Tool calls and agent subtasks are now queryable for reports/replay |

## Why this subset

PentAGI's strongest architectural pattern is not a specific tool, but the persistent execution hierarchy:

```text
Flow -> Task -> Subtask -> Action/ToolCall -> Reportable evidence
```

CTF-Agent already had sessions, specialist agents, reflection, summarization, knowledge extraction, and reporting. The biggest missing piece was durable, queryable execution state between a raw session and conversation messages. This pass adds that layer without replacing the existing solver loop.

## API additions

- `GET /api/sessions/{id}/subtasks`
- `GET /api/sessions/{id}/subtasks?status=success`
- `GET /api/sessions/{id}/tool-calls?limit=200`
- `GET /api/templates?type=web`
- `POST /api/templates`
- `GET /api/templates/{id}`
- `PUT /api/templates/{id}`
- `DELETE /api/templates/{id}`

## Default playbooks

The database seeds four baseline templates on migration:

- `web` — recon, discovery, hypothesis, exploit, extraction
- `pwn` — triage, reverse, primitive, exploit, extraction
- `crypto` — inventory, identify, attack, implement, extraction
- `reverse` — triage, locate, model, validate, extraction

## Next adaptation candidates

1. Add a Generator/Refiner loop that materializes planned subtasks before execution and patches the plan after every subtask.
2. Use the persisted tool calls to enrich reports instead of relying on replay files only.
3. Add tool-call statistics to the dashboard, grouped by tool name, agent, and challenge type.
4. Add optional template selection in the Solve UI so a user can start from a known playbook.

## Iteration 2: evidence-backed reports and tool-call analytics

The second pass closes the loop by consuming the persisted evidence:

- Session reports now prefer `subtasks` + `tool_calls` before falling back to replay files.
- Markdown reports include an **Agent 子任务** table when evidence exists.
- Dashboard payload now includes `tool_call_stats`.
- Added tool-call analytics endpoints:
  - `GET /api/tool-calls/stats`
  - `GET /api/sessions/{id}/tool-calls/stats`

This matches PentAGI's practical reporting idea: every action should be attributable to an agent, a subtask, a tool name, a status, and a timestamp.

## Iteration 3: deterministic Generator/Refiner planning

The third pass adds a lightweight planning layer inspired by PentAGI's Generator and Refiner agents while staying deterministic and offline-friendly:

- Generator: when `/api/solve` creates a session, CTF-Agent selects the first matching `flow_templates` entry and turns its numbered/bulleted checklist into `planner` subtasks with status `planned`.
- Refiner: when the solve finishes or fails, remaining `planned` subtasks are refined to `covered`, `success`, `needs_review`, or `skipped` based on the final session status and recorded tool-call evidence.
- New plan APIs:
  - `GET /api/sessions/{id}/plan`
  - `GET /api/sessions/{id}/plan?status=covered`
  - `POST /api/sessions/{id}/plan/refine`

This is intentionally not an LLM planner yet. It gives the UI/reporting layer a stable plan lifecycle first; a future LLM Generator/Refiner can replace the deterministic parser behind the same persisted model.

## Iteration 4: Refiner patch operations

The fourth pass adds a concrete patch API similar to PentAGI's `subtask_patch` flow. Plans now have a persistent `sort_order`, and callers can update a plan without regenerating all items.

New endpoint:

- `POST /api/sessions/{id}/plan/patch`

Patch shape:

```json
{
  "message": "Adjust plan after initial recon",
  "operations": [
    {"op": "modify", "id": 12, "description": "Focus on authenticated routes"},
    {"op": "add", "after_id": 12, "title": "JWT review", "description": "Inspect token claims and signing behavior"},
    {"op": "reorder", "id": 15, "after_id": 0},
    {"op": "remove", "id": 14}
  ]
}
```

Supported operations:

- `add`: requires `title` and `description`; optional `after_id` (`0` or absent means insert at beginning)
- `remove`: requires `id`
- `modify`: requires `id` plus `title` and/or `description`; resets the item to `planned`
- `reorder`: requires `id`; optional `after_id`

This gives the UI and future LLM Refiner a stable delta-update contract. The current deterministic Refiner and a future LLM Refiner can both emit the same patch format.

## Iteration 5: optional LLM Refiner suggestions

The fifth pass connects the patch contract to a refiner service. It can operate in two modes:

- deterministic mode: conservative offline suggestions from session status and tool-call evidence
- LLM mode: ask the configured LLM to emit the same patch JSON; any LLM failure falls back to deterministic mode

New endpoint:

- `POST /api/sessions/{id}/plan/suggest-patch`

Query options:

- `?llm=true` asks the configured model for the patch suggestion
- `?apply=true` applies the suggested patch immediately
- both can be combined: `?llm=true&apply=true`

The response includes the patch, the source (`deterministic`, `llm`, or `deterministic_fallback`), and optionally the updated plan when applied. This mirrors PentAGI's direction where a Refiner agent emits a patch, while keeping deterministic behavior available for offline CTF workflows.

## Iteration 6: optional LLM Generator

The sixth pass adds a Generator service for initial plans. Like the Refiner, it has a deterministic fallback path:

- deterministic mode: parse the selected `flow_templates` checklist into ordered plan steps
- LLM mode: ask the configured model to emit `{"steps":[{"title":"...","description":"..."}]}`; any error falls back to deterministic mode

`POST /api/solve` now accepts optional planning fields:

```json
{
  "challenge_type": "web",
  "description": "Find the flag",
  "target": "http://challenge.local",
  "plan_with_llm": true,
  "template_id": 1
}
```

New endpoint for existing sessions:

- `POST /api/sessions/{id}/plan/generate`

Query options:

- `?llm=true` uses the configured LLM for generation
- `?replace=true` deletes the existing planner-generated plan before creating a new one
- `?template_id=1` selects a specific template

The response includes `created`, `source` (`deterministic`, `llm`, or `deterministic_fallback`), and the resulting plan. This keeps the Generator/Refiner pair on one stable persistence model while allowing gradual LLM adoption.

## Iteration 7: frontend planning and evidence workflow

The seventh pass exposes the persisted planning layer in the React UI:

- Solve page:
  - users can choose a backend `flow_templates` entry for the selected challenge type;
  - users can enable optional LLM initial planning (`plan_with_llm`);
  - the submission log shows the number of generated planner subtasks and the source.
- Sessions page:
  - session details now load the planner plan, tool-call list, and tool-call stats;
  - users can regenerate a plan, regenerate with LLM, refine statuses, request a patch suggestion, or apply the deterministic/LLM suggestion;
  - tool-call evidence is visible beside the conversation for quick replay/report triage.
- Dashboard:
  - the global `tool_call_stats` payload is rendered as a tool evidence panel.

This turns the earlier backend-only PentAGI adaptation into an operator-facing loop: generate a plan, run the agent, inspect durable evidence, refine/patch the plan, then export a stronger report.
