import Link from "next/link";

export default function Home() {
  return (
    <main id="main" className="landing">
      <section className="hero">
        <h1>TODO.md was never built for a fleet.</h1>
        <p className="lede">
          Steering coding agents with TODO files breaks at fleet scale: parallel
          agents trample each other&apos;s work, sessions die and take their
          context with them, and nothing records what actually happened. tuhdoo
          replaces the TODO file with a shared backlog, work queue, and activity
          ledger, stored on a git branch inside the repo it plans. It syncs over
          the git remote you already have and needs no server, no vendor, and no
          accounts.
        </p>
        <div className="hero-actions">
          <Link className="button" href="/docs">
            Read the docs
          </Link>
          <code className="install-snippet">
            npm i -D tuhdoo &amp;&amp; npx tuhdoo init
          </code>
        </div>
      </section>

      <section className="section">
        <h2>One repo. One clone. One history.</h2>
        <p>
          The backlog and every agent&apos;s activity live on the{" "}
          <strong>data branch</strong>: an orphan git branch inside the repo,
          with its own history, carrying coordination data instead of code.
          Clone the repo and the whole plan comes with it; sync is a push and a
          pull over the remote you already have. There is no server to run, yet
          the plan is still distributed, version-tracked, and owned by whoever
          owns the repo.
        </p>
      </section>

      <section className="section">
        <h2>The agent loop</h2>
        <p>
          Agents connect over the{" "}
          <a href="https://modelcontextprotocol.io" rel="noopener">
            Model Context Protocol (MCP)
          </a>{" "}
          and work one recorded loop: claim, work, escalate, finish. The entire
          surface is twelve tools, few enough that the whole protocol fits in
          the single instruction file an agent harness — the tool that runs your
          agents — loads. Every step lands on the ledger, which is the only
          continuity between one session and the next.
        </p>
        <div className="card-grid">
          <div className="card">
            <h3>
              <span className="step">1</span>Claim
            </h3>
            <p>
              <code>claim_next</code> hands the agent the highest-priority ready
              task with its description, acceptance criteria, prior notes, and
              run history. A claim is an exclusive, time-boxed lease; if the
              agent dies, the task returns to the pool.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">2</span>Work
            </h3>
            <p>
              Work happens with ordinary git on ordinary branches; tuhdoo never
              touches how code is written or merged. Notes on the ledger
              checkpoint context for whoever picks the task up next.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">3</span>Escalate
            </h3>
            <p>
              When a question needs a human, the agent asks and hands off
              instead of guessing. A blocking escalation holds the task until
              the question is answered, and the next claimant receives the
              question and the answer together.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">4</span>Finish
            </h3>
            <p>
              Every run ends with a recorded outcome. Reporting{" "}
              <code>done</code> means the acceptance criteria actually hold, and
              a confirmation gate settles competing claims before anything
              merges.
            </p>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Your loop: capture, shape, drain</h2>
        <p>
          Capture ideas into the inbox the moment they occur — a title alone is
          enough, and capturing never drags you into a planning session. Later,
          work through the pile with an agent and shape captures into real tasks
          with full descriptions, acceptance criteria, and dependency edges.
          Then agents drain the ready pool while you watch progress from the
          tuhdoo TUI (terminal user interface) and answer escalations on your
          schedule, not the fleet&apos;s.
        </p>
      </section>

      <section className="section">
        <h2>No service attached</h2>
        <div className="card-grid">
          <div className="card">
            <h3>Nothing to run</h3>
            <p>
              tuhdoo ships as a binary and a git branch; there is no backend
              service, no subscription, no signup, and no vendor to trust. If
              you can clone the repo, you hold the entire coordination state,
              and everything works offline.
            </p>
          </div>
          <div className="card">
            <h3>One backlog for the team</h3>
            <p>
              Everyone who clones the repo sees the same backlog, whatever code
              branch they have checked out, and every action is attributed to
              the human behind it. The plan also renders as browsable markdown
              on your git host.
            </p>
          </div>
          <div className="card">
            <h3>Leaves no trace</h3>
            <p>
              Joining is a clone and one <code>init</code>; leaving is a handful
              of git commands. tuhdoo installs no hooks, writes nothing into
              your worktree, and puts no commits on your code branches.
            </p>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Get started</h2>
        <p>
          Install the single static binary from npm, a release archive, or{" "}
          <code>go install</code>. Run <code>tuhdoo init</code>, then connect
          your agent harness with one MCP config snippet.
        </p>
        <p>
          <Link className="button" href="/docs">
            Docs: joining a repo, the agent protocol, workflow recipes →
          </Link>
        </p>
      </section>
    </main>
  );
}
