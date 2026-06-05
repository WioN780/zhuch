import { Game } from "./core/Game.js";
import "./styles/main.css";

document.addEventListener("DOMContentLoaded", () => {
  const game = new Game();
  game.initialize().catch((err) => {
    console.error("Failed to initialize game:", err);
  });
});
