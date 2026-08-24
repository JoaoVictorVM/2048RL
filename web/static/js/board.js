export const BOARD_SIZE = 4;
export const SLIDE_MS = 110;

const LINE_SCANS = {
  left: () => rows(false),
  right: () => rows(true),
  up: () => cols(false),
  down: () => cols(true),
};

function rows(reverse) {
  const lines = [];
  for (let r = 0; r < BOARD_SIZE; r++) {
    const line = [];
    for (let c = 0; c < BOARD_SIZE; c++) {
      line.push([r, c]);
    }
    lines.push(reverse ? line.reverse() : line);
  }
  return lines;
}

function cols(reverse) {
  const lines = [];
  for (let c = 0; c < BOARD_SIZE; c++) {
    const line = [];
    for (let r = 0; r < BOARD_SIZE; r++) {
      line.push([r, c]);
    }
    lines.push(reverse ? line.reverse() : line);
  }
  return lines;
}

function emptyGrid() {
  return Array.from({ length: BOARD_SIZE }, () => Array(BOARD_SIZE).fill(null));
}

export function maxTile(board) {
  return board.reduce((best, row) => Math.max(best, ...row), 0);
}

// Renderiza o tabuleiro a partir do estado que o servidor devolve. O casamento
// entre tiles antigos e novos existe só para animar o deslize: nenhuma regra do
// jogo é decidida aqui: o estado é sempre o que veio de internal/game.
export class BoardRenderer {
  constructor(container) {
    this.container = container;
    this.board = null;
    this.tileAt = emptyGrid();
    this.#buildStructure();
  }

  #buildStructure() {
    this.container.classList.add('board');
    this.container.innerHTML = '';

    const grid = document.createElement('div');
    grid.className = 'board-grid';
    for (let i = 0; i < BOARD_SIZE * BOARD_SIZE; i++) {
      const cell = document.createElement('div');
      cell.className = 'board-cell';
      grid.appendChild(cell);
    }

    this.layer = document.createElement('div');
    this.layer.className = 'board-tiles';

    this.container.appendChild(grid);
    this.container.appendChild(this.layer);
  }

  reset(board) {
    this.layer.innerHTML = '';
    this.tileAt = emptyGrid();
    this.board = board.map((row) => [...row]);

    for (let r = 0; r < BOARD_SIZE; r++) {
      for (let c = 0; c < BOARD_SIZE; c++) {
        if (board[r][c] !== 0) {
          this.tileAt[r][c] = this.#createTile(board[r][c], r, c, 'tile-new');
        }
      }
    }
  }

  update(board, direction) {
    if (this.board === null || !LINE_SCANS[direction]) {
      this.reset(board);
      return;
    }

    const survivors = emptyGrid();
    const pending = [];
    const consumed = new Set();
    const previousTiles = this.tileAt.flat().filter((tile) => tile !== null);

    for (const line of LINE_SCANS[direction]()) {
      const previous = line
        .map(([r, c]) => this.tileAt[r][c])
        .filter((tile) => tile !== null);
      const next = line
        .filter(([r, c]) => board[r][c] !== 0)
        .map(([r, c]) => ({ value: board[r][c], row: r, col: c }));

      let i = 0;
      for (const target of next) {
        const first = previous[i];
        const second = previous[i + 1];

        if (first && first.value === target.value) {
          this.#moveTile(first, target.row, target.col);
          survivors[target.row][target.col] = first;
          consumed.add(first);
          i += 1;
        } else if (first && second && first.value === second.value && first.value * 2 === target.value) {
          this.#moveTile(first, target.row, target.col);
          this.#moveTile(second, target.row, target.col);
          consumed.add(first);
          consumed.add(second);
          pending.push({ ...target, className: 'tile-merged' });
          i += 2;
        } else {
          pending.push({ ...target, className: 'tile-new' });
        }
      }
    }

    this.board = board.map((row) => [...row]);
    this.tileAt = survivors;

    window.setTimeout(() => {
      for (const tile of previousTiles) {
        if (!consumed.has(tile)) {
          tile.el.remove();
        }
      }
      for (const item of pending) {
        this.tileAt[item.row][item.col] = this.#createTile(item.value, item.row, item.col, item.className);
      }
    }, SLIDE_MS);
  }

  #createTile(value, row, col, className) {
    const el = document.createElement('div');
    el.className = `tile ${className}`;
    el.dataset.value = String(value);
    if (value > 2048) {
      el.dataset.big = 'true';
    }
    el.textContent = String(value);
    el.style.setProperty('--row', String(row));
    el.style.setProperty('--col', String(col));
    this.layer.appendChild(el);
    return { value, row, col, el };
  }

  #moveTile(tile, row, col) {
    tile.row = row;
    tile.col = col;
    tile.el.style.setProperty('--row', String(row));
    tile.el.style.setProperty('--col', String(col));
  }
}
