import { BoardRenderer } from './board.js';

const elements = {
  board: document.getElementById('board'),
  run: document.getElementById('run'),
  checkpoint: document.getElementById('checkpoint'),
  score: document.getElementById('score'),
  moves: document.getElementById('moves'),
  episode: document.getElementById('episode'),
  status: document.getElementById('status'),
};

const renderer = new BoardRenderer(elements.board);

const state = {
  runs: [],
  socket: null,
  closingOnPurpose: false,
  reconnectUsed: false,
  board: null,
  episodes: 0,
};

function showStatus(message, tone = 'info') {
  elements.status.textContent = message ?? '';
  elements.status.dataset.tone = tone;
  elements.status.hidden = !message;
}

function selectedRun() {
  return state.runs.find((run) => run.run_id === elements.run.value) ?? null;
}

function renderCheckpointOptions() {
  const run = selectedRun();
  elements.checkpoint.innerHTML = '';

  if (!run) {
    return;
  }
  for (const checkpoint of [...run.checkpoints].reverse()) {
    const option = document.createElement('option');
    option.value = checkpoint.filename;
    option.textContent = `episódio ${checkpoint.episode}`;
    elements.checkpoint.appendChild(option);
  }
}

function latestCheckpointSelection(runs) {
  let best = null;
  for (const run of runs) {
    for (const checkpoint of run.checkpoints) {
      const modified = Date.parse(checkpoint.modified_at);
      if (best === null || modified > best.modified) {
        best = { runId: run.run_id, filename: checkpoint.filename, modified };
      }
    }
  }
  return best;
}

async function loadRuns() {
  const response = await fetch('/api/runs');
  if (!response.ok) {
    throw new Error(`não foi possível listar os treinos (${response.status})`);
  }

  const payload = await response.json();
  state.runs = (payload.runs ?? []).filter((run) => run.checkpoints.length > 0);

  elements.run.innerHTML = '';
  for (const run of state.runs) {
    const option = document.createElement('option');
    option.value = run.run_id;
    option.textContent = run.run_id;
    elements.run.appendChild(option);
  }

  const latest = latestCheckpointSelection(state.runs);
  if (latest) {
    elements.run.value = latest.runId;
    renderCheckpointOptions();
    elements.checkpoint.value = latest.filename;
  } else {
    renderCheckpointOptions();
  }

  const hasRuns = state.runs.length > 0;
  elements.run.disabled = !hasRuns;
  elements.checkpoint.disabled = !hasRuns;
  return hasRuns;
}

function streamURL() {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const url = new URL(`${scheme}://${window.location.host}/ws/live`);
  if (elements.run.value) {
    url.searchParams.set('run_id', elements.run.value);
  }
  if (elements.checkpoint.value) {
    url.searchParams.set('checkpoint', elements.checkpoint.value);
  }
  return url.toString();
}

function closeStream() {
  if (state.socket === null) {
    return;
  }
  state.closingOnPurpose = true;
  state.socket.close();
  state.socket = null;
}

function connect() {
  closeStream();
  state.closingOnPurpose = false;
  showStatus('Conectando ao agente...');

  const socket = new WebSocket(streamURL());
  state.socket = socket;

  socket.addEventListener('open', () => {
    state.reconnectUsed = false;
    showStatus('');
  });

  socket.addEventListener('message', (event) => {
    handleMessage(JSON.parse(event.data));
  });

  socket.addEventListener('close', () => {
    if (state.closingOnPurpose || state.socket !== socket) {
      return;
    }
    state.socket = null;

    if (state.reconnectUsed) {
      showStatus('Conexão perdida. Recarregue a página para voltar a assistir.', 'error');
      return;
    }
    state.reconnectUsed = true;
    showStatus('Conexão perdida, tentando reconectar...', 'warning');
    window.setTimeout(connect, 1000);
  });
}

function handleMessage(message) {
  switch (message.type) {
    case 'episode_start':
      state.episodes += 1;
      state.board = message.board;
      renderer.reset(message.board);
      elements.score.textContent = String(message.score);
      elements.moves.textContent = '0';
      elements.episode.textContent = `Episódio ${state.episodes} · ${message.run_id} · ${message.checkpoint}`;
      showStatus('');
      break;

    case 'move':
      state.board = message.board;
      renderer.update(message.board, message.direction);
      elements.score.textContent = String(message.score);
      elements.moves.textContent = String(message.move_count);
      break;

    case 'episode_end':
      showStatus(
        `Fim do episódio: score ${message.score}, maior tile ${message.max_tile}` +
          `, ${message.move_count} jogadas. Novo episódio em instantes...`,
      );
      break;

    case 'error':
      showStatus(`${message.message} (${message.code})`, 'error');
      break;
  }
}

elements.run.addEventListener('change', () => {
  renderCheckpointOptions();
  connect();
});

elements.checkpoint.addEventListener('change', connect);

async function start() {
  try {
    const hasRuns = await loadRuns();
    if (!hasRuns) {
      showStatus('Nenhum checkpoint disponível ainda. Rode um treino com go run ./cmd/train.', 'warning');
      return;
    }
  } catch (error) {
    showStatus(error.message, 'error');
    return;
  }
  connect();
}

start();
