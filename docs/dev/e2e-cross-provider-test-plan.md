# E2E Test Plan: Cross-Provider Pipeline (claude backend + codex frontend)
> Phase 3.3 verification — พิสูจน์ว่า claude + codex ทำงานร่วมกันใน AOM project เดียวได้

**วันที่**: 2026-05-26  
**เป้า**: backend-main (claude) + frontend-main (codex) ทำงานพร้อมกันใน project เดียว โดยไม่ collision และ verify gate ผ่านทั้งคู่  
**Environment**: WSL2 Ubuntu, claude + codex runtimes  
**Test mode**: `--real` (live sessions จริง ไม่ใช่ --mock)

---

## สิ่งที่ต้องพิสูจน์

| ประเด็น | เหตุผล |
|---------|--------|
| claude + codex ทำงานในโปรเจคเดียวได้ | workspace isolation (branch-per-agent) ต้องไม่ conflict |
| codex verify gate ผ่านโดยไม่ `--force` | verify checks ต้องอ่าน workspace `.agent/` ของ codex ได้ |
| merge ทั้ง 2 provider ขึ้น main ได้ | `[TASK-xxx]` commit tagging + merge commit pipeline |
| channel communication ข้ามค่าย | claude และ codex โพสต์เข้า channel.md เดียวกันได้ |

---

## Prerequisites

```bash
# ใน WSL
which claude   # ต้องเจอ
which codex    # ต้องเจอ
which aom      # ต้องเจอ (build จาก Windows ก่อน)

# Build binary สำหรับ Linux
# รันบน Windows (PowerShell):
# $env:GOOS='linux'; $env:GOARCH='amd64'; go build -o aom-linux cmd/aom/main.go
# cp aom-linux \\wsl$\Ubuntu\tmp\aom-e2e-xprovider\aom
```

---

## Setup

```bash
# ใน WSL
export PATH="/tmp/aom-e2e-xprovider:$PATH"
mkdir -p /tmp/aom-e2e-xprovider

# สร้างโปรเจคทดสอบ
mkdir -p /tmp/e2e-xprovider && cd /tmp/e2e-xprovider
git init && git config user.email "test@aom.local" && git config user.name "AOM Test"
echo "# Cross-Provider E2E Test" > README.md
git add README.md && git commit -m "init"

# init AOM project
aom project init --name "e2e-xprovider" --default-branch main

# ตรวจสอบ doctor
aom doctor
# ต้องผ่านทุก check รวมถึง WSL2 bypass, claude binary, codex binary
```

---

## Phase A: ตั้ง agents 2 providers

```bash
# backend: claude runtime
aom agent add backend-main --role backend --class builder --runtime claude

# frontend: codex runtime
aom agent add frontend-main --role frontend --class frontend --runtime codex

# reviewer: claude runtime (สำหรับ review รอบสุดท้าย)
aom agent add reviewer-main --role reviewer --class reviewer --runtime claude

# แก้ model ให้ประหยัด
aom agent set-model backend-main claude-haiku-4-5-20251001
aom agent set-model reviewer-main claude-haiku-4-5-20251001

# provision workspaces (สร้าง permanent git worktrees)
aom agent provision backend-main
aom agent provision frontend-main
aom agent provision reviewer-main

# ตรวจสอบ
aom agent list
git worktree list
# ควรเห็น 3 worktrees: agents/backend-main, agents/frontend-main, agents/reviewer-main
```

---

## Phase B: สร้าง tasks แยกต่างหากสำหรับแต่ละ agent

```bash
# Task 1: backend (claude)
BACK_TASK=$(aom task create "Build a simple Python HTTP server with GET /hello" \
  --agent backend-main --format json | jq -r '.id')
echo "Backend task: $BACK_TASK"

aom step add $BACK_TASK "Write server.py with Flask GET /hello returning JSON" --format json | jq -r '.id'
aom task ready $BACK_TASK

# Task 2: frontend (codex)
FRONT_TASK=$(aom task create "Build a simple HTML page that fetches GET /hello and displays the result" \
  --agent frontend-main --format json | jq -r '.id')
echo "Frontend task: $FRONT_TASK"

aom step add $FRONT_TASK "Write index.html with fetch() call and display result" --format json | jq -r '.id'
aom task ready $FRONT_TASK

# ตรวจสอบ
aom status
# ควรเห็น 2 tasks ใน Ready state, agent ต่างคนต่าง runtime
```

---

## Phase C: Spawn ทั้งสอง agent พร้อมกัน

```bash
# spawn backend (claude)
BACK_SESSION=$(aom session spawn backend-main --task $BACK_TASK --real --format json | jq -r '.id')
echo "Backend session: $BACK_SESSION"

# spawn frontend (codex) — หน้าต่างต่างกัน, provider ต่างกัน
FRONT_SESSION=$(aom session spawn frontend-main --task $FRONT_TASK --real --format json | jq -r '.id')
echo "Frontend session: $FRONT_SESSION"

# ส่งคำสั่งเริ่มต้นให้ทั้งคู่
aom session send $BACK_SESSION "read .agent/task.md and begin implementing the task"
aom session send $FRONT_SESSION "read .agent/task.md and begin implementing the task"

# ดู status แบบ live
aom dashboard --interval 15s
# หรือ
watch -n 15 "aom status; echo; aom channel read | tail -10"
```

**สิ่งที่ agent แต่ละตัวต้องทำ (ตาม profile):**

| Step | backend-main (claude) | frontend-main (codex) |
|------|----------------------|----------------------|
| 1 | อ่าน `.agent/task.md` | อ่าน `.agent/task.md` |
| 2 | เขียน `server.py` | เขียน `index.html` |
| 3 | อัปเดต `.agent/state.md` | อัปเดต `.agent/state.md` |
| 4 | `git commit -m "[TASK-xxx] ..."` | `git commit -m "[TASK-xxx] ..."` |
| 5 | `aom task signal task.completed --task $TASK_ID` | `aom task signal task.completed --task $TASK_ID` |
| 6 | `aom channel append "backend-main: done"` | `aom channel append "frontend-main: done"` |

---

## Phase D: Monitor และ verify ทั้งคู่

```bash
# ดู output ทั้งสอง session
aom capture backend-main --follow
aom capture frontend-main --follow

# ดู channel ว่า agent โพสต์ status อะไรบ้าง
aom channel read

# verify backend (claude)
aom task verify $BACK_TASK
# Expected:
# [ok]  commits on branch (agents/backend-main)
# [ok]  state.md updated
# [ok]  handoff.md filled (or waived)
# [ok]  task.completed in log

# verify frontend (codex)
aom task verify $FRONT_TASK
# Expected: same 4 checks, reading from .aom/agents/frontend-main/workspace/.agent/
```

**Note:** ถ้า verify fail ให้ดูที่:
```bash
# ดู workspace log ของแต่ละ agent
cat .aom/agents/backend-main/workspace/.agent/log.md
cat .aom/agents/frontend-main/workspace/.agent/log.md

# ดูว่า commit มี [TASK-xxx] prefix ไหม
git log agents/backend-main --oneline | head -5
git log agents/frontend-main --oneline | head -5
```

---

## Phase E: Accept ทั้งคู่

```bash
# accept backend
aom task accept $BACK_TASK

# accept frontend
aom task accept $FRONT_TASK

# ตรวจสอบ status
aom status
# ทั้งสอง task ควรเป็น Done
```

---

## Phase F: Merge ทั้งคู่ขึ้น main

```bash
# merge backend ก่อน
aom merge check $BACK_TASK
aom merge prepare $BACK_TASK
aom merge commit $BACK_TASK

# ตรวจสอบว่า backend commit ขึ้น main
git log --oneline main | head -5

# merge frontend
aom merge check $FRONT_TASK
aom merge prepare $FRONT_TASK
aom merge commit $FRONT_TASK

# ตรวจสอบว่าทั้งคู่ขึ้น main
git log --oneline main | head -10
```

---

## Checklist ที่ต้องผ่านทั้งหมด

```
Setup
[ ] aom project init สำเร็จ
[ ] aom doctor ผ่านทุก check (รวม codex wsl2 bypass + claude binary)
[ ] provision สร้าง workspace สำหรับทั้ง 3 agents
[ ] git worktree list แสดง agents/backend-main, agents/frontend-main, agents/reviewer-main

Backend (claude) workflow
[ ] session spawn backend-main --real ไม่ error
[ ] claude เขียน server.py จริงใน workspace
[ ] commit มี prefix [TASK-xxx]
[ ] state.md อัปเดตจาก "None recorded yet"
[ ] .agent/log.md มี "task.completed"
[ ] channel.md มีข้อความจาก backend-main

Frontend (codex) workflow
[ ] session spawn frontend-main --real ไม่ error
[ ] codex เขียน index.html จริงใน workspace
[ ] commit มี prefix [TASK-xxx]
[ ] state.md อัปเดต
[ ] .agent/log.md มี "task.completed"
[ ] channel.md มีข้อความจาก frontend-main

No collision
[ ] agents/backend-main และ agents/frontend-main ไม่มีไฟล์ชนกัน
[ ] aom doctor ไม่ warn เรื่อง workspace collision
[ ] ทั้งสอง session ทำงานพร้อมกันโดยไม่ขัดกัน

Verify gates (ทั้งคู่)
[ ] aom task verify $BACK_TASK → ทุก check [ok]
[ ] aom task verify $FRONT_TASK → ทุก check [ok]
[ ] aom task accept $BACK_TASK สำเร็จ (ไม่ต้องใช้ --force)
[ ] aom task accept $FRONT_TASK สำเร็จ (ไม่ต้องใช้ --force)

Merge
[ ] aom merge check ไม่ conflict สำหรับทั้งคู่
[ ] aom merge commit $BACK_TASK สำเร็จ
[ ] aom merge commit $FRONT_TASK สำเร็จ
[ ] git log main แสดง commits จากทั้ง backend + frontend จริง
```

---

## สัญญาณที่บอกว่า Phase 3.3 สำเร็จ

1. Checklist ด้านบนผ่านครบโดยไม่ต้องใช้ `--force` แม้แต่ครั้งเดียว
2. claude (backend) + codex (frontend) ทำงานพร้อมกันใน project เดียวโดยไม่ collision
3. verify gate อ่าน workspace artifacts ได้ทั้งสอง provider
4. `git log main` แสดง commit จากทั้ง `agents/backend-main` และ `agents/frontend-main`

---

## Expected failure modes (และวิธีแก้)

| Failure | สาเหตุ | วิธีแก้ |
|---------|--------|--------|
| `codex: binary not found` | codex ไม่ได้ install ใน WSL | `npm install -g @openai/codex` |
| `verify: task.completed not found` | codex ไม่ได้รัน `aom task signal` | ดู profile — task.md ต้องมี completion checklist |
| `verify: no [TASK-xxx] commits` | codex ไม่ได้ใส่ prefix | ดู frontend.md.tmpl commit convention section |
| `G1 collision error` | ทั้งสอง agent ใช้ runtime เดียวกันแต่ไม่มี workspace | ตรวจ `aom agent provision` ครบหรือเปล่า |
| `merge: empty commit set` | commit ไม่มี `[TASK-xxx]` prefix | verify จะจับก่อน merge — fix commit ก่อน |
| `codex bwrap error` | WSL2 sandbox conflict | WSL2 auto-detect ต้องเปิดแล้ว; ดู `aom doctor` |

---

## การรายงานผล

บันทึกผลใน `E2E-REPORT-WSL2-CROSS-PROVIDER.md` ตามโครงสร้างเดียวกับ `E2E-REPORT-WSL2-CLAUDE.md`:

```markdown
# AOM E2E Test Report — WSL2 Cross-Provider Pipeline (claude + codex)
Date: <วันที่>
Environment: Windows 11 + WSL2 Ubuntu
Agents: backend-main (claude-haiku) + frontend-main (codex)
...
```
