# AOM Master Plan
## แกนกลางของ Agent ทุกตัวใน Repo

**อัปเดตล่าสุด**: 2026-05-26  
**สถานะ**: Phase 1–5 สมบูรณ์ — Level 5 Guided Autonomy พร้อมใช้งาน

---

## วิสัยทัศน์

> AOM คือแกนหลังให้ agent ทุกตัวทำงานใน repo เดียวกันได้
> โดยไม่ชนกัน ไม่สูญหาย และ operator ควบคุมได้จากจุดเดียว

คุณสร้าง repo โปรเจค → รัน `aom project init` → spawn agent เท่าที่ต้องการ
จากนั้น agent แต่ละตัวทำงานใน native CLI ของตัวเอง (claude / codex / gemini / kiro)
AOM เป็นตัวกลางที่:

- **รู้** ว่า agent ทุกตัวทำอะไรอยู่ที่ไหน
- **กั้น** ไม่ให้ agent เขียนทับกัน
- **ส่ง** งานและบริบทให้ agent ถูกตัวถูกเวลา
- **ยืนยัน** ว่างานจริงๆ เสร็จก่อนรับ
- **ประสาน** git ให้ merge ได้โดยไม่ conflict

---

## สิ่งที่ AOM เป็น vs ไม่เป็น

| AOM เป็น | AOM ไม่เป็น |
|---------|------------|
| Control plane ของทีม agent | Runtime ของ agent (agent ใช้ native CLI ของตัวเอง) |
| ที่เก็บ state ของงาน (SQLite + markdown) | IDE หรือ editor |
| ตัวกั้น collision และ verify completion | Orchestrator อัตโนมัติแบบ autonomous |
| ช่องทางสื่อสารระหว่าง agent | Message broker ในระดับ infrastructure |
| Git coordinator สำหรับ merge | Version control system |

---

## สถาปัตยกรรมหลัก

```
Operator
   │
   ▼
 aom CLI ──────────────────────────────────────────────────
   │                                                       │
   ├── SQLite DB (.aom/sessions.db)          State source of truth
   ├── Markdown artifacts (.aom/tasks/<id>/) Durable memory
   │
   ├── tmux sessions ─────────────────────────────────────
   │       ├── agent: backend-1   (claude --runtime claude)
   │       ├── agent: frontend-1  (claude --runtime claude)
   │       ├── agent: reviewer-1  (claude --runtime claude)
   │       └── agent: backend-2   (codex)
   │
   └── git worktrees ─────────────────────────────────────
           ├── .aom/agents/backend-1/workspace/  (branch: agents/backend-1)
           ├── .aom/agents/frontend-1/workspace/ (branch: agents/frontend-1)
           ├── .aom/agents/reviewer-1/workspace/ (branch: agents/reviewer-1)
           └── worktrees/<task-id>/              (branch: aom/task-xxx-title)
```

### สามชั้นของความจริง (ลำดับความน่าเชื่อถือ)

```
1. .agent/*.md artifacts   ← source of truth หลัก (durable)
2. SQLite DB               ← structured state สำหรับ query
3. tmux sessions           ← ephemeral ทดแทนได้เสมอ
```

---

## สิ่งที่ทำงานได้แล้ว (Phase 1 Complete)

### Core Infrastructure
- [x] `aom project init` — สร้าง `.aom/` พร้อม config ครบ
- [x] Task + Step state machines (Draft → Done)
- [x] Artifact system — task.md / state.md / handoff.md / log.md / index.md
- [x] SQLite WAL mode + immediate lock + busy timeout

### Agent Management
- [x] `aom agent add/list/provision` — เพิ่มและ provision workspace
- [x] Per-agent permanent workspace (Free-Roam) — แต่ละ agent มี worktree ถาวร
- [x] G1/G2/G3 collision guards — ป้องกัน agent ชนกัน
- [x] `aom session spawn/stop/resume/attach` — lifecycle ครบ
- [x] Profile injection — agent รู้บทบาท, workflow, และ team context

### Operator Control
- [x] `aom status` — ดูสถานะทุก agent พร้อมกัน
- [x] `aom attach <session>` — โดดเข้า native CLI ของ agent ใดก็ได้
- [x] `aom capture [--all] [--follow]` — อ่าน output agent โดยไม่ต้องเปิด tmux
- [x] `aom session send` — ส่งคำสั่งหา agent จากภายนอก
- [x] `aom task verify` + `aom task accept` — gate ก่อนรับงาน

### Team Coordination
- [x] `aom channel append/read` — shared team channel
- [x] `aom message send/read` — direct message ระหว่าง agent
- [x] `aom broadcast` — ส่งข้อความทั้งทีม
- [x] `aom team brief` — สรุปสถานะทีมทั้งหมด
- [x] `aom next` — งานถัดไปที่พร้อมทำ เรียงลำดับ priority

### Git Coordination
- [x] Per-task worktree (traditional) หรือ per-agent workspace
- [x] `aom merge check/prepare/commit` — merge pipeline ครบ
- [x] `[TASK-xxx]` tag convention สำหรับ workspace agents
- [x] `aom worktree prune/read-file` — cross-worktree operations

### Provider Support
- [x] **claude** — full support, native session detection, WSL2 tested
- [x] **codex** — full support, WSL2 bwrap bypass, deny_commands fixed
- [ ] gemini — stub เท่านั้น (CLI flags unconfirmed)
- [ ] kiro — stub เท่านั้น (CLI flags unconfirmed)

---

## ช่องโหว่ที่รู้จักและแผนแก้

### ระดับ Critical (Phase 2)

| # | ปัญหา | ผลกระทบ | สถานะ |
|---|-------|---------|--------|
| F1 | ~~log.md ownership contradiction~~ | ~~completion signal เสียเงียบ~~ | ✅ แก้แล้ว 2026-05-26 |
| F2 | ~~verify ไม่เช็ค `[TASK-xxx]` prefix~~ | ~~commits หายตอน merge~~ | ✅ แก้แล้ว 2026-05-26 |
| F3 | ~~hardcoded "Out of Scope" ผิดบริบท~~ | ~~agent อ่านแล้วสับสน~~ | ✅ แก้แล้ว 2026-05-26 |

### ระดับ High (Phase 2)

| # | ปัญหา | สถานะ |
|---|-------|--------|
| F4 | ~~Two conflicting "starting session" protocols~~ | ✅ แก้แล้ว 2026-05-26 |
| F5 | ~~commit guard ใน `aom task show` ข้าม workspace agents~~ | ✅ แก้แล้ว 2026-05-26 |
| F6 | ~~verify checks syntactic ไม่ semantic~~ | ✅ แก้แล้ว 2026-05-26 |
| F7 | ~~orchestrator profile ไม่รู้ว่า accept อาจ fail~~ | ✅ แก้แล้ว 2026-05-26 |
| F8 | ~~frontend profile ไม่มี `[TASK-xxx]` convention~~ | ✅ แก้แล้ว 2026-05-26 |

---

## Roadmap

### Phase 1 — Foundation (สมบูรณ์แล้ว)
> เป้า: operator ใช้ AOM control หลาย agent ด้วยมือได้

- ✅ Provider: claude + codex
- ✅ Multi-worktree isolation
- ✅ Task/step pipeline
- ✅ Completion gate (verify + accept)
- ✅ Operator navigation (attach/capture/send)

---

### Phase 2 — Reliable Multi-Agent Handoff ✅ (สมบูรณ์ 2026-05-26)
> เป้า: workflow builder → reviewer ครบวงจรโดยไม่มี silent failure

**ผลลัพธ์**: E2E test ผ่านครบ — builder commit → verify 5/5 checks → accept → reviewer spawn → review-report.md → accept + aom merge commit ✅

```
✅ 2.1  แก้ F2: เพิ่ม [TASK-xxx] commit check ใน runTaskVerifyChecks
✅ 2.2  แก้ F4: รวม starting-session protocol ให้เหลือ 1 ลำดับ
✅ 2.3  แก้ F5: commit guard ใน aom task show ครอบ workspace agents
✅ 2.4  aom task signal <event> — CLI command สำหรับ agent ส่งสัญญาณ
✅ 2.5  E2E test: claude builder + claude reviewer ผ่านครบทุก check
✅ 2.6  docs/dev/e2e-2agent-test-plan.md เขียน script และ checklist
```

---

### Phase 3 — Provider Expansion ⚠️ (Partial)
> เป้า: ทุก provider ที่ประกาศไว้ทำงานได้จริง

```
⏳ 3.1  gemini: implement LaunchShellSpec — Blocked (CLI flags unconfirmed)
⏳ 3.2  kiro:   implement LaunchShellSpec — Blocked (CLI flags unconfirmed)
✅ 3.3  E2E test cross-provider: codex-be + claude-fe — 5/5 verify checks, merged (2026-05-26)
        residual: codex uses step.completed instead of task.completed as final signal
        operator unblock: aom task signal task.completed --task <id> --summary "..."
✅ 3.4  provider-specific profile tuning (builder/frontend/reviewer/orchestrator ครบ)
```

---

### Phase 4 — Operator UX: Navigation & Observability ✅ (สมบูรณ์ 2026-05-26)
> เป้า: operator jump ระหว่าง agent ได้ smooth และเห็น state ทั้งหมดในที่เดียว

```
✅ 4.1  aom switch <agent-name>         — jump ด้วย agent name ไม่ต้องรู้ session ID, auto-logs intervention
✅ 4.2  aom dashboard [--interval <d>]  — live terminal UI ทุก N วิ (sessions + action items + channel)
✅ 4.3  aom task verify --watch         — poll ทุก 10s จนทุก check ผ่าน, exit เมื่อ ready
✅ 4.4  aom status --action-items       — filter เฉพาะ APPROVAL / ACCEPT / SPAWN / BLOCKED items
```

---

### Phase 5 — Guided Autonomy ✅ (สมบูรณ์ 2026-05-26)
> เป้า: orchestrator agent ดูแลทีมได้โดยไม่ต้องการ operator ทุกขั้นตอน

**ผลลัพธ์**: 3 คำสั่งใหม่ + escalation hints ครบ — 24 tests ผ่านทั้งหมด — `go build ./...` clean

```
✅ 5.1  aom task accept --auto [--interval 15s] [--timeout 30m]
        — polling loop: รอทุก check ผ่าน → accept อัตโนมัติ; timeout → escalation hints

✅ 5.2  aom session watch [--auto-spawn] [--interval 15s] [--timeout 60m] [--real|--mock]
        — ดู Ready tasks; --auto-spawn: spawn agent อัตโนมัติเมื่อ task พร้อม

✅ 5.3  aom run-pipeline <task-id> [--agent] [--timeout] [--real|--mock] [--skip-merge]
        — one-command 5 stages: spawn → wait(task.completed) → verify → accept → merge

✅ 5.4  timeout + escalation built-in ทุก polling command
        — พิมพ์ remaining budget + คำสั่ง resume เฉพาะ stage เมื่อ timeout

✅ 5.5  cross-provider E2E (3.3) — codex-be + claude-fe ผ่าน 5/5 checks (2026-05-26)
```

---

## เกณฑ์วัดว่า "พร้อมเป็นแกนกลาง"

```
Level 1 — Supervised ✅
  operator spawn agent → monitor → accept ด้วยมือ
  ทำงานได้กับ claude + codex
  
Level 2 — Reliable Handoff ✅ (Phase 2 สมบูรณ์ 2026-05-26)
  builder → reviewer pipeline ไม่เสียเงียบๆ
  verify gate จับ incomplete work ได้ก่อน accept
  aom task signal แทนการเขียน log.md มือ
  E2E test ผ่านครบโดยไม่ใช้ --force

Level 3 — Multi-Provider ⚠️ (Phase 3 partial)
  claude ✅ / codex ✅ / gemini ⏳ / kiro ⏳
  cross-provider E2E ✅ ผ่าน 5/5 checks (2026-05-26)
  residual: codex compliance gap (step.completed vs task.completed)

Level 4 — Fluid Navigation ✅ (Phase 4 สมบูรณ์ 2026-05-26)
  aom switch <agent-name> — jump ใน 1 command
  aom dashboard — ทุก agent ใน 1 หน้าจอ, refresh อัตโนมัติ
  aom status --action-items — เห็น todo list ชัดเจน
  aom task verify --watch — รอจนผ่านโดยไม่ต้องรัน manual

Level 5 — Guided Autonomy ✅ (Phase 5 สมบูรณ์ 2026-05-26)
  aom task accept --auto — รอแล้ว accept อัตโนมัติ
  aom session watch --auto-spawn — spawn agents เมื่อ task Ready โดยไม่ต้องรอ operator
  aom run-pipeline — one-command ครบ 5 stages พร้อม timeout + escalation
  operator intervention เฉพาะเมื่อ escalate มาเท่านั้น
```

---

## สิ่งที่จะไม่ทำใน AOM

- **ไม่เป็น IDE** — agent ใช้ native CLI ของตัวเอง AOM ไม่แทรกแซง runtime
- **ไม่ซ่อน state** — ทุกอย่างอ่านได้จาก `.aom/` และ `.agent/` โดยตรง
- **ไม่เป็น fully autonomous pipeline** — human operator ยังอยู่ในวง loop เสมอ
- **ไม่ผูกกับ provider เดียว** — ออกแบบให้เพิ่ม provider ใหม่ได้โดยไม่แก้ core
- **ไม่ทำ speculative features** — implement เฉพาะสิ่งที่มี use case พิสูจน์แล้ว

---

## คำถามที่ยังเปิดอยู่

1. **`aom task signal` command** — ควรมี CLI command ให้ agent ส่งสัญญาณแทนการเขียน log.md โดยตรง? จะแก้ F1 ได้ถาวรและทำให้ schema ownership ชัดเจน
2. **Orchestrator as agent** — orchestrator ควรเป็น agent ที่รัน AOM commands หรือควรเป็น operator ที่เขียน script? Trade-off ระหว่าง flexibility กับ predictability
3. **gemini/kiro timeline** — รอ CLI flags จาก provider หรือทำ community research?
4. **Profile length** — ควรแยก base profile เป็นหลาย section ที่ inject ตาม context แทนที่จะ inject ทั้งหมดทุกครั้ง?

---

## Reference

| ไฟล์ | อธิบาย |
|------|--------|
| `docs/AOM-planning.md` | Product vision และ scope assumptions |
| `docs/AOM-milestones.md` | Milestone breakdown |
| `docs/state-machine.md` | State lifecycle ครบทุก entity |
| `docs/artifact-schemas.md` | Markdown artifact contracts |
| `docs/cli-spec.md` | CLI command specifications |
| `docs/dev/current-status.md` | Implementation progress handoff |
| `E2E-REPORT-WSL2-CLAUDE.md` | E2E test report (WSL2, claude-haiku, 2026-05-26) |
