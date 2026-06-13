import { TICK_MS } from './constants.js';
import { clearBoard, drawCell } from './board.js';
import { Snake } from './snake.js';
import { spawnFood } from './food.js';

export class Game {
  constructor(ctx) {
    this.ctx = ctx;
    this.snake = new Snake();
    this.food = spawnFood(this.snake);
    this.score = 0;
    this.running = false;
    this._timerId = null;
  }

  start() {
    this.running = true;
    this._timerId = setInterval(() => this._tick(), TICK_MS);
  }

  stop() {
    this.running = false;
    clearInterval(this._timerId);
    this._timerId = null;
  }

  setDirection(dir) {
    this.snake.setDirection(dir);
  }

  _tick() {
    this.snake.move();

    if (this.snake.collidesWithSelf()) {
      this.stop();
      this._draw();
      return;
    }

    const head = this.snake.head();
    if (head.x === this.food.x && head.y === this.food.y) {
      this.snake.grow();
      this.score += 1;
      this.food = spawnFood(this.snake);
    }

    this._draw();
  }

  _draw() {
    clearBoard(this.ctx);
    for (const seg of this.snake.body) {
      drawCell(this.ctx, seg.x, seg.y, '#4caf50');
    }
    drawCell(this.ctx, this.food.x, this.food.y, '#f44336');
  }
}
