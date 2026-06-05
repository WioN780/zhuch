import { Application } from "pixi.js";
import { Socket } from "./Socket.js";
import { Renderer } from "../rendering/Renderer.js";
import { UIManager } from "../ui/UIManager.js";
import { InputManager } from "./InputManager.js";

export class Game {
  constructor() {
    this.app = null;
    this.socket = null;
    this.renderer = null;
    this.ui = null;
    this.input = null;

    this.state = "INITIALIZING"; // INITIALIZING, MENU, CONNECTING, PLAYING, ERROR
  }

  async initialize() {
    // Initialize PixiJS Application
    this.app = new Application();
    await this.app.init({
      resizeTo: window,
      backgroundColor: 0x0a0a0a,
      antialias: true,
      resolution: window.devicePixelRatio || 1,
      autoDensity: true,
    });

    document.getElementById("game-container").appendChild(this.app.canvas);

    // Initialize Managers
    this.ui = new UIManager(this);
    this.renderer = new Renderer(this);
    this.input = new InputManager(this);
    this.socket = new Socket(this);

    // Set initial state
    this.setState("MENU");

    // Start main loop
    this.app.ticker.add((ticker) => {
      this.update(ticker.deltaTime);
    });
  }

  setState(newState) {
    console.log(`Game state: ${this.state} -> ${newState}`);
    this.state = newState;
    this.ui.onStateChange(newState);
  }

  update(deltaTime) {
    if (this.state === "PLAYING") {
      this.renderer.update(deltaTime);
      this.input.update(deltaTime);
    }
  }

  async connect(playerName, roomID = "default") {
    this.setState("CONNECTING");
    try {
      await this.socket.connect(playerName, roomID);
      this.setState("PLAYING");
    } catch (err) {
      console.error("Connection failed:", err);
      this.setState("ERROR");
      this.ui.showError("Connection failed. Please try again.");
    }
  }
}
