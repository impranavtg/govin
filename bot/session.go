package bot

import (
	"sync"
	"time"
)

// --- Session State for Conversational Flows ---

type Session struct {
	Action      string
	Step        int
	Description string
	Amount      float64
	PaidBy      string
	Date        time.Time
	MessageID   int
}

var (
	sessionMu    sync.RWMutex
	userSessions = make(map[int64]*Session)
)

func getSession(chatID int64) *Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return userSessions[chatID]
}

func setSession(chatID int64, s *Session) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	userSessions[chatID] = s
}

func clearSession(chatID int64) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(userSessions, chatID)
}
