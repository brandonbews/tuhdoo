#!/usr/bin/env python3
"""tuhdoo TUI mockups — generated so column math is exact.

Emits, for each variant: <name>.ansi (real colors, cat it in a terminal)
and <name>.txt (plain; bar lines filled with ░ so structure survives).

Geometry (80 cols):
  mark(2) + id(6) + gap(2) + pri(2) + gap(2) = 14 -> title column at 14
"""

W = 80
TITLE_COL = 14
TITLE_W = W - TITLE_COL

R = "\x1b[0m"
BOLD = "\x1b[1m"
DIM = "\x1b[2m"
REV = "\x1b[7m"
RED = "\x1b[31m"
GREEN = "\x1b[32m"
YELLOW = "\x1b[33m"
MAGENTA = "\x1b[35m"
BG = {  # black text on color
    "magenta": "\x1b[30;45m",
    "green": "\x1b[30;42m",
    "yellow": "\x1b[30;43m",
    "red": "\x1b[30;41m",
}

ready = [
    ("t-vv29", "p1", "npm devDependency distribution (esbuild-pattern wrapper packages)", ["distribution", "npm"]),
    ("t-rqjm", "p1", "Arm the TUI detail screen: selectable escalation, enter to answer; p/c on the viewed task", ["cli", "tui"]),
    ("t-73kw", "p1", "Needs Input: enter answers in place; blocked rows stop repeating the question", ["cli", "tui", "ux"]),
    ("t-4gmz", "p1", "TUI navigation: up/down arrows move the cursor; footer says so", ["cli", "tui", "ux"]),
    ("t-q5ev", "p1", "Rename the cancel interaction: archive as the human verb, task.cancelled stays the plumbing", ["cli", "tui", "ux", "design"]),
    ("t-v9vk", "p1", "TUI mouse support: click selects, click again acts as enter", ["cli", "tui", "ux"]),
    ("t-qagh", "p1", "Align MCP tool descriptions with the revised notes doctrine", ["protocol", "docs", "mcp"]),
    ("t-wrkm", "p1", "Brand the task IDs: mint tuh-, accept both prefixes, age out t-", ["cli", "tui", "ux", "design"]),
    ("t-p213", "p1", "CLI write verbs: a paved path when no MCP session exists", ["cli", "dx"]),
    ("t-3a4d", "p0", "Daemon portability: unix-only lock and the socket-path length limit", ["go", "platform", "parked"]),
    ("t-sy1p", "p0", "Tree/parent-grouped rendering in the TUI list", ["cli", "tui", "design"]),
]

blocked = [
    ("t-gsw5", "v0 definition of done: the dogfood week holds",
     "escalation: v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06."),
    ("t-qm7a", "v1 milestone: steering surface and a second machine",
     "depends on t-gsw5"),
    ("t-cb1j", "Two-machine dogfood: real claim races over one remote",
     "escalation: This task is human-paced — it needs you on a second machine running fleets for a week."),
    ("t-frth", "Epoch compaction (D9): snapshot event + in-commit deletion",
     "depends on t-qm7a"),
]

escalations = [
    ("t-gsw5", "v0 definition-of-done check: did the dogfood week hold? Answer on or after 2026-08-06.",
     "brandon/migrator", "2026-07-30 04:28 UTC"),
    ("t-cb1j", "This task is human-paced — it needs you on a second machine running fleets for a week — so an agent can't execute it. When do you want to start the week?",
     "brandon/impl-2", "2026-07-30 05:51 UTC"),
]


def ell(s, n):
    return s if len(s) <= n else s[: n - 1] + "…"


def title_cell(title, labels):
    lab = "  [" + ", ".join(labels) + "]" if labels else ""
    if len(title) + len(lab) <= TITLE_W:
        return ell(title, TITLE_W - len(lab)), lab
    # labels lose first: drop them rather than eat the title
    if len(title) >= TITLE_W - 10:
        return ell(title, TITLE_W), ""
    return title, ell(lab, TITLE_W - len(title))


class Out:
    def __init__(self):
        self.ansi, self.txt = [], []

    def raw(self, ansi_line, txt_line):
        self.ansi.append(ansi_line)
        self.txt.append(txt_line)

    def line(self, parts):
        """parts: list of (text, sgr-or-None) — txt gets text only."""
        a = "".join(t if s is None else f"{s}{t}{R}" for t, s in parts)
        self.raw(a, "".join(t for t, _ in parts))

    def blank(self):
        self.raw("", "")

    def bar(self, text, sgr, right=""):
        pad = W - len(text) - len(right)
        content = text + " " * max(pad, 1) + right
        self.raw(f"{sgr}{content[:W]}{R}", (text + " " + "░" * max(pad - 2, 0) + " " + right)[:W])


def task_row(o, id_, pri, title, labels, cursor=False, pri_sgr=None):
    t, lab = title_cell(title, labels)
    mark = "▸ " if cursor else "  "
    o.line([
        (mark, BOLD if cursor else None),
        (id_, DIM), ("  ", None),
        (pri.ljust(2), pri_sgr), ("  ", None),
        (t, BOLD if cursor else None), (lab, DIM),
    ])


def esc_rows(o, task, question, actor, raised, cursor=False):
    mark = "▸ " if cursor else "  "
    o.line([
        (mark, BOLD if cursor else None),
        (task, DIM), ("  ", None),
        ("! ", RED + BOLD), ("  ", None),
        (ell(question, TITLE_W), BOLD if cursor else None),
    ])
    o.line([(" " * TITLE_COL, None), ("blocking", RED), (f" · {actor} · {raised}", DIM)])


def blocked_rows(o, id_, title, reason, cursor=False):
    task_row(o, id_, "", title, [], cursor=cursor)
    o.line([(" " * TITLE_COL, None), ("waiting: ", RED), (ell(reason, TITLE_W - 9), DIM)])


def sections(o, bar):
    bar("NEEDS INPUT", 2, "magenta", "a answer")
    for i, e in enumerate(escalations):
        esc_rows(o, *e, cursor=(i == 0))
    o.blank()
    bar("READY", 11, "green", "p priority · c archive")
    for r in ready:
        task_row(o, *r, pri_sgr=(YELLOW if r[1] == "p0" else DIM))
    o.blank()
    bar("IN PROGRESS", 0, "yellow", "")
    o.line([("  ", None), ("none", DIM)])
    o.blank()
    bar("BLOCKED", 4, "red", "")
    for b in blocked:
        blocked_rows(o, *b)


def variant_a():
    """Colored section bars, htop-style header/footer bars."""
    o = Out()
    o.bar(" tuhdoo · syncing with origin · fetched 2026-07-31 14:02 UTC", REV + BOLD,
          right="acting as brandon ")
    o.blank()

    def bar(label, n, color, hint):
        o.bar(f" {label} ({n})", BG[color], right=(hint + " " if hint else ""))
    sections(o, bar)
    o.blank()
    o.bar(" j/k move · enter open · a answer · p priority · c archive · q quit", REV + DIM,
          right="11 done ")
    return o


def variant_b():
    """One neutral bar style (reverse video) everywhere; color lives in rows."""
    o = Out()
    o.bar(" tuhdoo · syncing with origin · fetched 2026-07-31 14:02 UTC", REV + BOLD,
          right="acting as brandon ")
    o.blank()

    def bar(label, n, color, hint):
        o.bar(f" {label} ({n})", REV, right=(hint + " " if hint else ""))
    sections(o, bar)
    o.blank()
    o.line([("j/k move · enter open · a answer · p priority · c archive · q quit", DIM)])
    return o


def variant_c():
    """Austere: dim rules instead of filled bars; fixed columns do the work."""
    o = Out()
    o.line([("tuhdoo", BOLD), (" · syncing with origin · fetched 14:02 UTC · acting as ", None),
            ("brandon", BOLD)])
    o.blank()
    colors = {"magenta": MAGENTA, "green": GREEN, "yellow": YELLOW, "red": RED}

    def bar(label, n, color, hint):
        head = f"─ {label} ({n}) "
        rule = "─" * (W - len(head))
        o.raw(f"{DIM}─ {R}{BOLD}{colors[color]}{label}{R} {DIM}({n}) {rule}{R}", head + rule)
    sections(o, bar)
    o.blank()
    o.line([("j/k move · enter open · a answer · p priority · c archive · q quit", DIM)])
    return o


def current():
    """Faithful-enough reproduction of today's render for side-by-side."""
    o = Out()
    o.line([("tuhdoo", BOLD), (" · sync: syncing with \"origin\" · last fetch 2026-07-31 14:02 UTC · acting as ", None), ("brandon", BOLD)])
    o.blank()
    o.line([("11 ready", GREEN), (" · ", None), ("0 in progress", YELLOW), (" · ", None),
            ("4 blocked", RED), (" · 11 done · 0 cancelled · 2 escalations", None)])
    o.blank()
    o.line([("Needs Input", BOLD), (" (2)", None)])
    first = True
    for task, q, actor, raised in escalations:
        if not first:
            o.blank()
        first = False
        o.line([("▸ " if first else "  ", None), (ell(q, 76), BOLD), ("  [blocking]", RED)])
        o.line([(f"      task {task} (…title…) · asked by {actor} · raised {raised}", None)])
        first = False
    o.blank()
    o.line([("Ready", BOLD + GREEN), (" (11)", None)])
    first = True
    for id_, pri, title, labels in ready[:4]:
        if not first:
            o.blank()
        first = False
        o.line([("  ", None), (id_, DIM), (f"  {pri}  ", None), (ell(title, 60), None),
                ("  [" + ", ".join(labels) + "]", None)])
    o.line([("  …", DIM)])
    return o


import sys, os
outdir = os.path.dirname(os.path.abspath(__file__))
for name, fn in [("current", current), ("mock-a", variant_a), ("mock-b", variant_b), ("mock-c", variant_c)]:
    o = fn()
    with open(f"{outdir}/{name}.ansi", "w") as f:
        f.write("\n".join(o.ansi) + "\n")
    with open(f"{outdir}/{name}.txt", "w") as f:
        f.write("\n".join(o.txt) + "\n")
print("wrote", outdir)
