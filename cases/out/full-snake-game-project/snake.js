import { GRID_SIZE } from './constants.js';

export class Snake {
  constructor() {
    this.body = [
      { x: 10, y: 10 },
      { x: 9, y: 10 },
      { x: 8, y: 10 },
    ];
    this.direction = { x: 1, y: 0 };
    this._pendingGrow = false;
  }

  setDirection(dir) {
    if (dir.x === -this.direction.x && dir.y === -this.direction.y) return;
    this.direction = dir;
  }

  head() {
    return this.body[0];
  }

  move() {
    const h = this.head();
    const next = {
      x: (h.x + this.direction.x + GRID_SIZE) % GRID_SIZE,
      y: (h.y + this.direction.y + GRID_SIZE) % GRID_SIZE,
    };
    this.body.unshift(next);
    if (this._pendingGrow) {
      this._pendingGrow = false;
    } else {
      this.body.pop();
    }
  }

  grow() {
    this._pendingGrow = true;
  }

  collidesWithSelf() {
    const h = this.head();
    return this.body.slice(1).some(seg => seg.x === h.x && seg.y === h.y);
  }
}
