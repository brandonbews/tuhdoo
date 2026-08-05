package syncer

import (
	"errors"
	"time"

	"github.com/brandonbews/tuhdoo/internal/gitx"
)

// adoptTimeout bounds the one startup fetch AdoptRemoteBranch makes. An
// unreachable remote must not hang daemon startup (T2: remoteless — and
// by extension offline — is a normal state); a bound this generous only
// ever cuts off a genuinely dead transport, and losing the adoption is
// harmless (see below).
const adoptTimeout = 10 * time.Second

// AdoptRemoteBranch is the clone-join path, run once at daemon startup
// before store.Init: when the local data branch does not exist but the
// configured remote already carries one, fetch it and adopt it as the
// local ref — so a fresh clone joins the existing history instead of
// minting a second, unrelated orphan root.
//
// Best-effort by design, and deliberately so: on any failure — no
// remote configured, remote unreachable, remote without the branch —
// it logs (at most) and returns, and store.Init mints a fresh root
// exactly as before. That is always safe: two machines initializing
// before either pushes produce two roots no matter what, and the
// app-level union merge converges them (D6/T2) — adoption only keeps
// the common case to a single root. Correctness never rests here.
func (s *Syncer) AdoptRemoteBranch() {
	if _, err := s.git.ReadRef(s.ref); err == nil {
		return // local branch exists; nothing to adopt
	}
	if _, err := s.git.RemoteURL(s.remote); err != nil {
		// No remote is the normal remoteless state (T2); anything else
		// is config trouble the sync loop will surface on its own.
		if !errors.Is(err, gitx.ErrNoRemote) {
			s.logf("adopt: reading remote %q: %v; minting a fresh root", s.remote, err)
		}
		return
	}

	// Fetch into the syncer's own tracking ref, bounded so a dead
	// remote cannot stall startup, then create the local ref with a
	// must-not-exist CAS — never clobbering a branch that appeared
	// concurrently.
	err := s.git.FetchTimeout(s.remote, s.ref+":"+TrackingRef, adoptTimeout)
	if err != nil {
		if errors.Is(err, gitx.ErrRemoteRefMissing) {
			return // remote has no data branch yet; mint as usual
		}
		s.logf("adopt: fetch from %q failed (%v); minting a fresh root — the sync merge reconciles later", s.remote, err)
		return
	}
	head, err := s.git.ReadRef(TrackingRef)
	if err != nil {
		s.logf("adopt: reading fetched head: %v; minting a fresh root", err)
		return
	}
	err = s.git.UpdateRef(s.ref, head, "")
	if err != nil {
		if !errors.Is(err, gitx.ErrRefCASFailed) {
			s.logf("adopt: setting %s: %v; minting a fresh root", s.ref, err)
		}
		return
	}
	s.logf("adopt: joined existing data branch from %q at %s", s.remote, head)
}
