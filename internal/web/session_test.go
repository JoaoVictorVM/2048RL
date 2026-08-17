package web

import (
	"sync"
	"testing"
	"time"

	"github.com/JoaoVictorVM/2048RL/internal/game"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore(time.Minute)
	g := game.NewGame()

	created, err := store.Create(g)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Token == "" {
		t.Fatal("esperado um token não vazio")
	}
	if len(created.Token) != 36 {
		t.Errorf("token deveria ter formato UUID, obtido %q", created.Token)
	}

	found, ok := store.Get(created.Token)
	if !ok {
		t.Fatal("a sessão recém-criada deveria ser encontrada")
	}
	if found != created || found.Game != g {
		t.Error("a sessão retornada não corresponde à criada")
	}
	if store.Len() != 1 {
		t.Errorf("esperada 1 sessão no store, obtidas %d", store.Len())
	}
}

func TestSessionStore_TokensAreUnique(t *testing.T) {
	store := NewSessionStore(time.Minute)

	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		session, err := store.Create(game.NewGame())
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[session.Token] {
			t.Fatalf("token repetido: %s", session.Token)
		}
		seen[session.Token] = true
	}
}

func TestSessionStore_UnknownTokenNotFound(t *testing.T) {
	store := NewSessionStore(time.Minute)

	if _, ok := store.Get("token-que-nunca-existiu"); ok {
		t.Error("esperado não encontrar um token desconhecido")
	}
}

func TestSessionStore_ExpiresAfterTTL(t *testing.T) {
	now := time.Now()
	store := NewSessionStore(30 * time.Minute)
	store.now = func() time.Time { return now }

	session, err := store.Create(game.NewGame())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	now = now.Add(29 * time.Minute)
	if _, ok := store.Get(session.Token); !ok {
		t.Fatal("a sessão não deveria expirar antes do TTL")
	}

	now = now.Add(31 * time.Minute)
	if removed := store.Sweep(); removed != 1 {
		t.Errorf("esperada 1 sessão removida pela varredura, removidas %d", removed)
	}
	if _, ok := store.Get(session.Token); ok {
		t.Error("a sessão expirada não deveria mais ser recuperável")
	}
	if store.Len() != 0 {
		t.Errorf("esperado store vazio, restaram %d sessões", store.Len())
	}
}

func TestSessionStore_GetRenewsActivity(t *testing.T) {
	now := time.Now()
	store := NewSessionStore(30 * time.Minute)
	store.now = func() time.Time { return now }

	session, err := store.Create(game.NewGame())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for i := 0; i < 5; i++ {
		now = now.Add(20 * time.Minute)
		if _, ok := store.Get(session.Token); !ok {
			t.Fatalf("iteração %d: a atividade deveria renovar o TTL", i)
		}
	}

	if removed := store.Sweep(); removed != 0 {
		t.Errorf("uma sessão ativa não deveria ser varrida, removidas %d", removed)
	}
}

func TestSessionStore_GetDropsExpiredSessionOnRead(t *testing.T) {
	now := time.Now()
	store := NewSessionStore(time.Minute)
	store.now = func() time.Time { return now }

	session, err := store.Create(game.NewGame())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, ok := store.Get(session.Token); ok {
		t.Fatal("a sessão expirada não deveria ser retornada")
	}
	if store.Len() != 0 {
		t.Errorf("a leitura deveria descartar a sessão expirada, restaram %d", store.Len())
	}
}

func TestSessionStore_ConcurrentAccessIsSafe(t *testing.T) {
	store := NewSessionStore(time.Minute)

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				session, err := store.Create(game.NewGame())
				if err != nil {
					t.Errorf("Create: %v", err)
					return
				}
				if found, ok := store.Get(session.Token); ok {
					found.Lock()
					found.Game.ApplyMove(game.Left)
					found.Unlock()
				}
				store.Sweep()
				store.Len()
			}
		}()
	}

	wg.Wait()
	if store.Len() != workers*50 {
		t.Errorf("esperadas %d sessões, obtidas %d", workers*50, store.Len())
	}
}
