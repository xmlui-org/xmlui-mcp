package xmluimcp

import (
	"sync"
)

// SessionManager manages multiple sessions
type SessionManager struct {
	sessions map[string]*SessionContext
	mutex    sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*SessionContext),
	}
}
