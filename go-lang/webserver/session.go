package webserver

import (
	"fmt"
	"sync"
	"time"

	"boardgame/engine"
	"boardgame/game"
	"boardgame/layout"

	"github.com/google/uuid"
)

// GameSession represents an active game session
type GameSession struct {
	ID          string
	Controller  *game.GameController
	HumanPlayer string

	// Map geometry for this session's board, validated against it at creation.
	// LayoutRaw is the file as read, so it can be served without re-marshalling.
	Layout    *layout.Layout
	LayoutRaw []byte

	driver *engine.Driver

	CreatedAt      time.Time
	LastAccessedAt time.Time
	mutex          sync.RWMutex
}

// NewGameSession creates a new game session
func NewGameSession(controller *game.GameController, humanPlayer string) *GameSession {
	now := time.Now()
	return &GameSession{
		ID:             uuid.New().String(),
		Controller:     controller,
		HumanPlayer:    humanPlayer,
		CreatedAt:      now,
		LastAccessedAt: now,
	}
}

// Touch updates the last accessed time
func (gs *GameSession) Touch() {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()
	gs.LastAccessedAt = time.Now()
}

// Lock serialises a request against a session.
//
// Every handler reaches through the session into one shared *game.GameController
// and mutates it. The mutex previously guarded only the timestamp, so two
// concurrent requests on the same game raced the entire object graph -- and a
// game is exactly the sort of thing a browser hits twice at once, since the
// client polls state on a timer while the player is clicking.
//
// One lock at the dispatch point rather than thirty inside the handlers: a
// request either owns the game or waits for it.
func (gs *GameSession) Lock()   { gs.mutex.Lock() }
func (gs *GameSession) Unlock() { gs.mutex.Unlock() }

// isHumanTurnLocked answers without taking the mutex, for callers that already
// hold it. A sync.RWMutex is not reentrant, so a handler running under Lock
// cannot call the exported IsHumanTurn.
func (gs *GameSession) isHumanTurnLocked() bool {
	return gs.Controller.Game.CurrentPower == gs.HumanPlayer
}

// IsExpired checks if the session has expired (24 hours of inactivity)
func (gs *GameSession) IsExpired() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return time.Since(gs.LastAccessedAt) > 24*time.Hour
}

// IsHumanTurn checks if it's the human player's turn
func (gs *GameSession) IsHumanTurn() bool {
	gs.mutex.RLock()
	defer gs.mutex.RUnlock()
	return gs.isHumanTurnLocked()
}

// SessionManager manages all active game sessions
type SessionManager struct {
	sessions map[string]*GameSession
	mutex    sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*GameSession),
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new game session
func (sm *SessionManager) CreateSession(controller *game.GameController, humanPlayer string) (*GameSession, error) {
	// Validate that the human player exists in the game
	if _, exists := controller.Game.Players[humanPlayer]; !exists {
		return nil, fmt.Errorf("player %s not found in game", humanPlayer)
	}

	session := NewGameSession(controller, humanPlayer)

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.sessions[session.ID] = session
	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*GameSession, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	if session.IsExpired() {
		return nil, fmt.Errorf("session %s has expired", sessionID)
	}

	session.Touch()
	return session, nil
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.sessions[sessionID]; !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	delete(sm.sessions, sessionID)
	return nil
}

// GetSessionCount returns the number of active sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.sessions)
}

// cleanupExpiredSessions periodically removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		sm.mutex.Lock()
		for id, session := range sm.sessions {
			if session.IsExpired() {
				delete(sm.sessions, id)
			}
		}
		sm.mutex.Unlock()
	}
}


// Driver returns this session's turn driver, building it on first use.
//
// Handlers go through the driver rather than poking the controller directly, so
// the rules for ending a phase exist in exactly one place.
func (gs *GameSession) Driver() *engine.Driver {
	if gs.driver == nil {
		gs.driver = engine.New(gs.Controller)
	}
	return gs.driver
}
