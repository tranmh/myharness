import { Snake } from './snake.js';

const KEY_MAP = {
  ArrowUp:    { x: 0, y: -1 },
  ArrowDown:  { x: 0, y:  1 },
  ArrowLeft:  { x: -1, y: 0 },
  ArrowRight: { x:  1, y: 0 },
  w:          { x: 0, y: -1 },
  s:          { x: 0, y:  1 },
  a:          { x: -1, y: 0 },
  d:          { x:  1, y: 0 },
};

export function attachInput(snake) {
  window.addEventListener('keydown', (e) => {
    const dir = KEY_MAP[e.key];
    if (!dir) return;
    e.preventDefault();
    snake.setDirection(dir);
  });
}
