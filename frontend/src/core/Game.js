import { Application } from "pixi.js";
import { Socket } from "./Socket.js";
import { Renderer } from "../rendering/Renderer.js";
import { UIManager } from "../ui/UIManager.js";
import { InputManager } from "./InputManager.js";
import { CONFIG } from "./Config.js";

export class Game {
  constructor() {
    this.app = null;
    this.socket = null;
    this.renderer = null;
    this.ui = null;
    this.input = null;

    // Current active config (starts with defaults, updated by server)
    this.config = JSON.parse(JSON.stringify(CONFIG));

    this.state = "INITIALIZING"; // INITIALIZING, MENU, CONNECTING, PLAYING, ERROR
  }

  async initialize() {
    // Initialize PixiJS Application
    this.app = new Application();
    await this.app.init({
      resizeTo: window,
      backgroundColor: this.config.VISUALS.BACKGROUND_COLOR,
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
      this.update(ticker.deltaTime, ticker.deltaMS);
    });
  }

  // Called when server sends 'init' or config update
  applyServerConfig(serverConfig) {
    if (!serverConfig) return;

    // Map server keys to our internal config structure
    // This handles both camelCase and snake_case from Go backend
    if (serverConfig.WorldSize || serverConfig.world_size) {
      this.config.WORLD.SIZE =
        serverConfig.WorldSize || serverConfig.world_size;
    }

    const physics = this.config.PHYSICS;
    physics.FRICTION =
      serverConfig.Friction || serverConfig.friction || physics.FRICTION;
    physics.ACCELERATION =
      serverConfig.MoveAcceleration ||
      serverConfig.move_acceleration ||
      physics.ACCELERATION;
    physics.MAX_SPEED =
      serverConfig.MaxSpeed || serverConfig.max_speed || physics.MAX_SPEED;

    console.log("Applied server config:", this.config);

    // Notify renderer if world size changed
    if (this.renderer) {
      this.renderer.setupBackground();
    }
  }

  setState(newState) {
    console.log(`Game state: ${this.state} -> ${newState}`);
    this.state = newState;
    this.ui.onStateChange(newState);
  }

  update(deltaTime, deltaMS) {
    if (this.state === "PLAYING") {
      this.renderer.update(deltaTime, deltaMS);
      this.input.update(deltaTime);
    }
  }

  async connect(playerName, roomID = "default", customURL = null) {
    this.setState("CONNECTING");
    try {
      await this.socket.connect(playerName, roomID, customURL);
      this.setState("PLAYING");
    } catch (err) {
      console.error("Connection failed:", err);
      this.setState("ERROR");
      this.ui.showError("Connection failed. Please try again.");
    }
  }
}
