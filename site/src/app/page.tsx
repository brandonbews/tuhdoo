import Link from "next/link";

export default function Home() {
  return (
    <main className="landing">
      <section className="hero">
        <h1>TODO.md was never built for a fleet.</h1>
        <p className="lede">
          Steering coding agents today is markdown files and vibes: parallel
          agents trample each other, sessions die with their context, nothing
          records what happened. tuhdoo is the fix — a shared backlog, work
          queue, and activity ledger on a git branch inside the repo it plans.
          No server, no vendor, no accounts.
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
          The backlog, the roadmap, and every agent&apos;s activity are the same
          organism as the application. They live on the{" "}
          <strong>data branch</strong> — an orphan git branch, its own history,
          coordination data instead of code — inside the repo it plans. Clone
          the repo and the plan comes with it; sync is push and pull over the
          remote you already have. Serverless, yet still distributed,
          version-tracked, and owned by whoever owns the repo.
        </p>
      </section>

      <section className="section">
        <h2>The agent loop</h2>
        <p>
          Agents connect over a twelve-verb{" "}
          <a href="https://modelcontextprotocol.io" rel="noopener">
            MCP
          </a>{" "}
          surface and run one loop, recorded on the ledger — the only continuity
          between today&apos;s agent and tomorrow&apos;s.
        </p>
        <div className="card-grid">
          <div className="card">
            <h3>
              <span className="step">1</span>Claim
            </h3>
            <p>
              <code>claim_next</code> serves the highest-priority ready task,
              fully hydrated. A claim is an exclusive, time-boxed lease — it
              lapses back to the pool if the agent dies.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">2</span>Work
            </h3>
            <p>
              Ordinary git on ordinary branches — tuhdoo never touches how code
              gets written and merged. Notes checkpoint context for a successor.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">3</span>Escalate
            </h3>
            <p>
              An escalation is a question routed to a human — ask and hand off
              instead of guessing. A blocking escalation fences the task until
              answered; the next claimant inherits question and answer.
            </p>
          </div>
          <div className="card">
            <h3>
              <span className="step">4</span>Finish
            </h3>
            <p>
              Every run ends with a refereed outcome — <code>done</code> means
              the acceptance criteria actually hold, settled by a confirmation
              gate before anything merges.
            </p>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Your loop: capture, sculpt, drain</h2>
        <p>
          Ideas hit the inbox the moment they occur — title-only is fine;
          capture never costs a planning session. Later, sit down with an agent
          and sculpt the pile into workable tasks: prompt-quality descriptions,
          acceptance criteria, dependency edges. Then watch the drain from the
          TUI as agents work the ready frontier. You shape the graph and answer
          escalations on your schedule — never the fleet&apos;s.
        </p>
      </section>

      <section className="section">
        <h2>Yours. No service attached.</h2>
        <div className="card-grid">
          <div className="card">
            <h3>Nothing to run</h3>
            <p>
              No service, no subscription, no signup, no vendor to trust. If you
              can clone the repo, you hold the entire coordination state.
              Offline just works.
            </p>
          </div>
          <div className="card">
            <h3>One brain for the team</h3>
            <p>
              Everyone who clones sees the same backlog, whatever branch anyone
              has checked out — no asking who has what where — and every action
              is attributed to the human behind it. The plan renders as
              browsable markdown on your git host.
            </p>
          </div>
          <div className="card">
            <h3>Leaves no trace</h3>
            <p>
              Joining is a clone and one <code>init</code>; leaving is a handful
              of git commands — no hooks, no writes to your worktree, no commits
              on your code branches.
            </p>
          </div>
        </div>
      </section>

      <section className="section">
        <h2>Why I built it</h2>
        <blockquote className="pull">
          <p>
            All tuhdoo really does is let me see and organize the work while
            slightly slowing my agents down — and the slowdown is the feature:
            everything moves through typed, visible transitions I can steer. The
            plan and the code are one organism. My loop is capture an idea,
            sculpt it into a real task with an agent, watch the queue drain. My
            team pulls the same plan I do — nobody asks who has what on which
            branch. No service underneath — nothing to subscribe to, sign up
            for, or lose.
          </p>
          <footer>— Brandon, tuhdoo&apos;s maintainer</footer>
        </blockquote>
      </section>

      <section className="section">
        <h2>Get started</h2>
        <p>
          Install the single static binary — npm, release archive, or{" "}
          <code>go install</code> — run <code>tuhdoo init</code>, and connect
          your harness with one MCP snippet.
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
