import Link from "next/link";

export default function Home() {
  return (
    <main className="landing">
      <section className="hero">
        <h1>A coordination fabric for agent fleets, steered by humans.</h1>
        <p className="lede">
          tuhdoo is a shared backlog, work queue, and activity ledger that lives
          on a git branch inside the repo it plans — synced through an ordinary
          git remote. No server, no vendor, no accounts.
        </p>
        <div className="hero-actions">
          <Link className="button" href="/docs">
            Read the docs
          </Link>
          <code className="install-snippet">npm i -D tuhdoo &amp;&amp; npx tuhdoo init</code>
        </div>
      </section>

      <section className="section">
        <h2>What it is</h2>
        <p>
          Point an agent fleet at a backlog and the hard part isn&apos;t the
          work — it&apos;s seeing the work. tuhdoo keeps every task, claim,
          question, and outcome on one shared ledger. Agents connect through a
          twelve-verb <a href="https://modelcontextprotocol.io" rel="noopener">MCP</a>{" "}
          surface: they <strong>claim</strong> a task (an exclusive, time-boxed
          lease that stops two agents from building the same thing), work it on
          ordinary git branches, <strong>escalate</strong> questions to a human
          when they hit a wall, and finish by reporting an honest outcome.
          Humans steer from a terminal UI and CLI: capture ideas, triage them,
          set priorities and dependencies, and answer escalations — on their
          own schedule, not the fleet&apos;s.
        </p>
        <blockquote className="pull">
          <p>
            All it&apos;s really doing is letting me see and organize the work
            while slightly slowing agents down. The slowdown is the feature:
            everything moves through typed, visible transitions a human can
            steer.
          </p>
          <footer>— why tuhdoo exists</footer>
        </blockquote>
      </section>

      <section className="section">
        <h2>How it works</h2>
        <p>
          One loop, enforced by leases and recorded on the ledger. Sessions end
          and contexts compact; the ledger is the only continuity between
          today&apos;s agent and tomorrow&apos;s.
        </p>
        <div className="card-grid">
          <div className="card">
            <h3>
              <span className="step">1</span>Claim
            </h3>
            <p>
              An agent calls <code>claim_next</code> and gets the
              highest-priority ready task, fully hydrated — description,
              acceptance criteria, prior runs and notes. The lease renews
              automatically while the session lives; if the agent dies, the
              task returns to the pool.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">2</span>Work
            </h3>
            <p>
              Ordinary git on ordinary branches — tuhdoo never touches your
              code workflow. Optional notes checkpoint anything a successor
              would need if the session ends mid-flight.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">3</span>Escalate
            </h3>
            <p>
              Hit a wall? The agent raises an escalation — a question routed to
              a human — and hands off instead of guessing. A blocking
              escalation keeps the task out of the pool until someone answers;
              then any agent picks up question and answer together.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">4</span>Finish
            </h3>
            <p>
              Every run ends with a refereed outcome — <code>done</code> means
              the acceptance criteria actually hold, and a confirmation gate
              settles ownership before anything merges. No outcome an agent
              didn&apos;t earn.
            </p>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Why there&apos;s no server</h2>
        <p>
          The whole ledger is an <em>orphan branch</em> — a git branch with its
          own history, carrying coordination data instead of code — inside the
          repository it plans. Syncing is git syncing: push and pull over the
          remote you already have. That buys properties a coordination server
          has to work for:
        </p>
        <div className="card-grid">
          <div className="card">
            <h3>Nothing to run</h3>
            <p>
              No service to deploy, no accounts to create, no vendor to trust.
              If you can clone the repo, you have the entire coordination
              state — history included.
            </p>
          </div>
          <div className="card">
            <h3>Offline is normal</h3>
            <p>
              Everything works locally with no remote reachable; the first sync
              afterwards converges histories automatically. That&apos;s the
              normal operating mode, not a repair.
            </p>
          </div>
          <div className="card">
            <h3>Leaves no trace</h3>
            <p>
              Joining is a clone and one <code>init</code>; leaving is a
              handful of ordinary git commands. No hooks, no writes to your
              worktree, no commits on your code branches.
            </p>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Get started</h2>
        <p>
          Install the single static binary — via npm, a release archive, or{" "}
          <code>go install</code> — run <code>tuhdoo init</code> in the repo you
          want to plan, and connect your agent harness with one MCP snippet.
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
