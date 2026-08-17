import { BoardRenderer, SLIDE_MS, maxTile } from './board.js';

const KEY_DIRECTIONS = {
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
};

const elements = {
  board: document.getElementById('board'),
  score: document.getElementById('score'),
  best: document.getElementById('best'),
  newGame: document.getElementById('new-game'),
  newGameOverlay: document.getElementById('new-game-overlay'),
  overlay: document.getElementById('game-over'),
  finalScore: document.getElementById('final-score'),
  finalMaxTile: document.getElementById('final-max-tile'),
  reference: document.getElementById('reference'),
  status: document.getElementById('status'),
};

const renderer = new BoardRenderer(elements.board);

const state = {
  sessionId: null,
  board: null,
  score: 0,
  best: Number(window.localStorage.getItem('rl2048.best') ?? 0),
  gameOver: false,
  busy: false,
};

function showStatus(message) {
  elements.status.textContent = message ?? '';
  elements.status.hidden = !message;
}

function renderScore() {
  elements.score.textContent = String(state.score);
  elements.best.textContent = String(state.best);
}

function rememberBest() {
  if (state.score > state.best) {
    state.best = state.score;
    window.localStorage.setItem('rl2048.best', String(state.best));
  }
}

async function requestJSON(url, options) {
  const response = await fetch(url, options);
  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    const message = payload?.error?.message ?? `falha na requisição (${response.status})`;
    throw new Error(message);
  }
  return payload;
}

async function newGame() {
  state.busy = true;
  showStatus('');
  elements.overlay.hidden = true;

  try {
    const payload = await requestJSON('/api/human/new', { method: 'POST' });
    state.sessionId = payload.session_id;
    state.board = payload.board;
    state.score = payload.score;
    state.gameOver = payload.game_over;

    renderer.reset(payload.board);
    renderScore();
  } catch (error) {
    showStatus(`Não foi possível iniciar uma partida: ${error.message}`);
  } finally {
    state.busy = false;
  }
}

async function move(direction) {
  if (state.busy || state.gameOver || state.sessionId === null) {
    return;
  }

  state.busy = true;
  try {
    const payload = await requestJSON('/api/human/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: state.sessionId, direction }),
    });

    if (payload.moved) {
      renderer.update(payload.board, direction);
      state.board = payload.board;
      state.score = payload.score;
      renderScore();
    }

    state.gameOver = payload.game_over;
    if (state.gameOver) {
      rememberBest();
      renderScore();
      window.setTimeout(showGameOver, SLIDE_MS * 2);
    }
  } catch (error) {
    showStatus(`Jogada não aplicada: ${error.message}`);
  } finally {
    state.busy = false;
  }
}

async function showGameOver() {
  elements.finalScore.textContent = String(state.score);
  elements.finalMaxTile.textContent = String(maxTile(state.board));
  elements.reference.hidden = true;
  elements.overlay.hidden = false;

  try {
    const reference = await requestJSON('/api/human/reference');
    if (reference.available) {
      elements.reference.textContent =
        `Agente treinado (${reference.run_id}): score médio ${Math.round(reference.avg_score)}` +
        `, maior tile médio ${Math.round(reference.avg_max_tile)}`;
      elements.reference.hidden = false;
    }
  } catch {
    elements.reference.hidden = true;
  }
}

window.addEventListener('keydown', (event) => {
  const direction = KEY_DIRECTIONS[event.key];
  if (!direction) {
    return;
  }
  event.preventDefault();
  move(direction);
});

elements.newGame.addEventListener('click', newGame);
elements.newGameOverlay.addEventListener('click', newGame);

newGame();
