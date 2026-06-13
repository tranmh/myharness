import { Game } from './game.js';
import { attachInput } from './input.js';

const canvas = document.getElementById('board');
const ctx = canvas.getContext('2d');
const scoreEl = document.getElementById('score');

const game = new Game(ctx);

game.onScoreChange = (score) => {
  scoreEl.textContent = score;
};

attachInput(game);

game.start();
