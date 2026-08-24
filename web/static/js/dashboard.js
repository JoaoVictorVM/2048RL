const ACTIVE_RUN_THRESHOLD_MS = 30_000;
const POLL_INTERVAL_MS = 5_000;

const CHART_DEFINITIONS = [
  { id: 'win-rate', label: 'Win rate', color: '#edc22e', key: 'win_rate', percent: true },
  { id: 'avg-score', label: 'Score médio', color: '#f2b179', key: 'avg_score' },
  { id: 'avg-max-tile', label: 'Maior tile médio', color: '#8f7a66', key: 'avg_max_tile' },
];

const elements = {
  run: document.getElementById('run'),
  summary: document.getElementById('summary'),
  status: document.getElementById('status'),
  live: document.getElementById('live'),
};

const state = {
  runs: [],
  charts: new Map(),
  pollTimer: null,
};

function showStatus(message, tone = 'info') {
  elements.status.textContent = message ?? '';
  elements.status.dataset.tone = tone;
  elements.status.hidden = !message;
}

function selectedRun() {
  return state.runs.find((run) => run.run_id === elements.run.value) ?? null;
}

function isTraining(run) {
  if (!run?.metrics_modified_at) {
    return false;
  }
  return Date.now() - Date.parse(run.metrics_modified_at) < ACTIVE_RUN_THRESHOLD_MS;
}

function chartFor(definition) {
  if (state.charts.has(definition.id)) {
    return state.charts.get(definition.id);
  }

  const canvas = document.getElementById(definition.id);
  const chart = new Chart(canvas, {
    type: 'line',
    data: {
      labels: [],
      datasets: [
        {
          label: definition.label,
          data: [],
          borderColor: definition.color,
          backgroundColor: definition.color,
          borderWidth: 2,
          pointRadius: 2,
          tension: 0.25,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: { title: { display: true, text: 'episódio' } },
        y: {
          beginAtZero: true,
          ticks: definition.percent
            ? { callback: (value) => `${Math.round(value * 100)}%` }
            : undefined,
        },
      },
      plugins: { legend: { display: false } },
    },
  });

  state.charts.set(definition.id, chart);
  return chart;
}

function renderCharts(payload) {
  const labels = payload.points.map((point) => point.window_end);

  for (const definition of CHART_DEFINITIONS) {
    const chart = chartFor(definition);
    chart.data.labels = labels;
    chart.data.datasets[0].data = payload.points.map((point) => point[definition.key]);
    chart.update();
  }
}

function renderSummary(payload) {
  if (payload.episode_count === 0) {
    elements.summary.textContent = 'Esse treino ainda não gravou nenhum episódio.';
    return;
  }
  const last = payload.points.at(-1);
  elements.summary.textContent =
    `${payload.episode_count} episódios · janelas de ${payload.window_size} · ` +
    `última janela: win rate ${(last.win_rate * 100).toFixed(1)}%, ` +
    `score médio ${Math.round(last.avg_score)}, maior tile médio ${Math.round(last.avg_max_tile)}`;
}

async function fetchJSON(url) {
  const response = await fetch(url);
  const payload = await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(payload?.error?.message ?? `falha na requisição (${response.status})`);
  }
  return payload;
}

async function loadMetrics() {
  const run = selectedRun();
  if (!run) {
    return;
  }

  try {
    const payload = await fetchJSON(`/api/runs/${encodeURIComponent(run.run_id)}/metrics`);
    renderCharts(payload);
    renderSummary(payload);
    showStatus('');
  } catch (error) {
    showStatus(error.message, 'error');
  }
}

async function refreshRunState() {
  try {
    const payload = await fetchJSON('/api/runs');
    state.runs = payload.runs ?? [];
  } catch {
    // mantém a lista anterior: o polling seguinte tenta de novo
  }
}

function schedulePolling() {
  if (state.pollTimer !== null) {
    window.clearInterval(state.pollTimer);
    state.pollTimer = null;
  }

  const run = selectedRun();
  elements.live.hidden = !isTraining(run);
  if (!isTraining(run)) {
    return;
  }

  state.pollTimer = window.setInterval(async () => {
    await refreshRunState();
    await loadMetrics();
    const stillTraining = isTraining(selectedRun());
    elements.live.hidden = !stillTraining;
    if (!stillTraining) {
      window.clearInterval(state.pollTimer);
      state.pollTimer = null;
    }
  }, POLL_INTERVAL_MS);
}

async function selectRun() {
  for (const chart of state.charts.values()) {
    chart.data.labels = [];
    chart.data.datasets[0].data = [];
    chart.update();
  }
  elements.summary.textContent = '';

  await loadMetrics();
  schedulePolling();
}

elements.run.addEventListener('change', selectRun);

async function start() {
  try {
    await refreshRunState();
  } catch (error) {
    showStatus(error.message, 'error');
    return;
  }

  elements.run.innerHTML = '';
  for (const run of state.runs) {
    const option = document.createElement('option');
    option.value = run.run_id;
    option.textContent = run.has_metrics ? run.run_id : `${run.run_id} (sem métricas)`;
    elements.run.appendChild(option);
  }

  if (state.runs.length === 0) {
    elements.run.disabled = true;
    showStatus('Nenhum treino encontrado. Rode go run ./cmd/train para gerar métricas.', 'warning');
    return;
  }

  elements.run.disabled = false;
  const withMetrics = state.runs.filter((run) => run.has_metrics);
  if (withMetrics.length > 0) {
    elements.run.value = withMetrics.at(-1).run_id;
  }
  await selectRun();
}

start();
