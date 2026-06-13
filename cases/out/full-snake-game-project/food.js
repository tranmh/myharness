import { GRID_SIZE } from './constants.js';

export function spawnFood(snake) {
  let pos;
  do {
    pos = {
      x: Math.floor(Math.random() * GRID_SIZE),
      y: Math.floor(Math.random() * GRID_SIZE),
    };
  } while (snake.body.some(seg => seg.x === pos.x && seg.y === pos.y));
  return pos;
}
