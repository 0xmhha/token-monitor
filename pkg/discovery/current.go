package discovery

import (
	"errors"
	"os"
	"time"
)

const cacheTTL = time.Second

// FindCurrentSession implements Discoverer.FindCurrentSession.
//
// Detection priority:
//  1. CLAUDE_SESSION_ID env var → scan all baseDirs for matching session ID
//  2. CLAUDE_PROJECT_DIR env var → most recent .jsonl in that dir; if none found, fall through
//  3. Fallback → most recently modified .jsonl across all baseDirs
func (d *discoverer) FindCurrentSession() (*SessionFile, error) {
	if cached := d.getCached(); cached != nil {
		return cached, nil
	}

	session, err := d.detectCurrentSession()
	if err != nil {
		return nil, err
	}

	d.setCached(session)
	return session, nil
}

// getCached returns the cached session if still valid, nil otherwise.
func (d *discoverer) getCached() *SessionFile {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()

	if d.currentCache != nil && time.Since(d.cacheTime) < cacheTTL {
		return d.currentCache
	}
	return nil
}

// setCached updates the cache with the given session.
func (d *discoverer) setCached(session *SessionFile) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()

	d.currentCache = session
	d.cacheTime = time.Now()
}

// detectCurrentSession performs the actual session detection without caching.
func (d *discoverer) detectCurrentSession() (*SessionFile, error) {
	if sessionID := os.Getenv("CLAUDE_SESSION_ID"); sessionID != "" {
		if session := d.findBySessionID(sessionID); session != nil {
			return session, nil
		}
		d.logger.Debug("CLAUDE_SESSION_ID set but session not found, falling through",
			"session_id", sessionID)
	}

	if projectDir := os.Getenv("CLAUDE_PROJECT_DIR"); projectDir != "" {
		session, err := d.findMostRecentInDir(projectDir)
		if err == nil {
			return session, nil
		}
		if !errors.Is(err, ErrNoCurrentSession) {
			return nil, err
		}
		d.logger.Debug("CLAUDE_PROJECT_DIR set but no sessions found, falling through",
			"project_dir", projectDir)
	}

	return d.findMostRecentOverall()
}

// findBySessionID scans all baseDirs for a session matching the given ID.
// Returns nil if not found.
func (d *discoverer) findBySessionID(sessionID string) *SessionFile {
	sessions, err := d.Discover()
	if err != nil {
		d.logger.Warn("failed to discover sessions while looking up session ID",
			"session_id", sessionID, "error", err)
		return nil
	}

	for i := range sessions {
		if sessions[i].SessionID == sessionID {
			return &sessions[i]
		}
	}
	return nil
}

// findMostRecentInDir returns the most recently modified session in the given directory.
func (d *discoverer) findMostRecentInDir(projectDir string) (*SessionFile, error) {
	expanded := expandHome(projectDir)
	sessions, err := d.scanProjectDirectory(expanded)
	if err != nil {
		return nil, err
	}

	return mostRecent(sessions)
}

// findMostRecentOverall returns the most recently modified session across all baseDirs.
func (d *discoverer) findMostRecentOverall() (*SessionFile, error) {
	sessions, err := d.Discover()
	if err != nil {
		return nil, err
	}

	return mostRecent(sessions)
}

// mostRecent returns a pointer to the session with the highest ModTime.
// Returns ErrNoCurrentSession if the slice is empty.
func mostRecent(sessions []SessionFile) (*SessionFile, error) {
	if len(sessions) == 0 {
		return nil, ErrNoCurrentSession
	}

	best := &sessions[0]
	for i := 1; i < len(sessions); i++ {
		if sessions[i].ModTime > best.ModTime {
			best = &sessions[i]
		}
	}
	return best, nil
}
