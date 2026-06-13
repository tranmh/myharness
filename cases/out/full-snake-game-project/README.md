# Snake

A classic Snake game built with vanilla JavaScript ES modules and an HTML5 Canvas.

## Source Files

| File | Description |
|------|-------------|
| `index.html` | Entry point — sets up the page, score display, and canvas element |
| `styles.css` | Layout and visual styling |
| `main.js` | Bootstraps the game; imports all modules and starts the loop |
| `game.js` | Core game loop — manages state, timing, and win/lose logic |
| `snake.js` | Snake entity — position, movement, growth, and collision detection |
| `food.js` | Food entity — random placement and respawn logic |
| `board.js` | Canvas rendering — draws the grid, snake, and food each frame |
| `input.js` | Keyboard event handling — maps arrow/WASD keys to direction changes |
| `constants.js` | Shared configuration values (grid size, speed, colors, etc.) |

## Controls

| Key | Action |
|-----|--------|
| Arrow Up / W | Move up |
| Arrow Down / S | Move down |
| Arrow Left / A | Move left |
| Arrow Right / D | Move right |

## How to Run

Because `main.js` uses ES modules (`type="module"`), the game **must be served over HTTP** — browsers block module imports on `file://` URLs.

Any static file server works:
