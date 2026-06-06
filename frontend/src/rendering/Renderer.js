import { Container, Graphics } from "pixi.js";
import { Camera } from "./Camera.js";
import { EntityManager } from "../entities/EntityManager.js";

export class Renderer {
  constructor(game) {
    this.game = game;
    this.app = game.app;

    this.playerID = null;

    // Layers
    this.worldContainer = new Container();
    this.app.stage.addChild(this.worldContainer);

    this.backgroundLayer = new Container();
    this.entitiesLayer = new Container();
    this.effectsLayer = new Container();

    this.worldContainer.addChild(this.backgroundLayer);
    this.worldContainer.addChild(this.entitiesLayer);
    this.worldContainer.addChild(this.effectsLayer);

    this.camera = new Camera(this);
    this.entityManager = new EntityManager(this);

    // TPS Tracking
    this.lastUpdateTimestamp = performance.now();
    this.currentTPS = 0;
    this.tpsFilter = 0.9; // Smoothing factor

    this.setupBackground();
  }

  setupBackground() {
    const grid = new Container();
    this.backgroundLayer.addChild(grid);

    const size = 2000;
    const spacing = 100;

    const graphics = new Graphics();
    graphics.clear();

    // Main grid lines
    for (let x = 0; x <= size; x += spacing) {
      graphics.moveTo(x, 0);
      graphics.lineTo(x, size);
    }
    for (let y = 0; y <= size; y += spacing) {
      graphics.moveTo(0, y);
      graphics.lineTo(size, y);
    }

    graphics.stroke({ color: 0xffffff, width: 1, alpha: 0.03 });

    // Border
    graphics.moveTo(0, 0);
    graphics.lineTo(size, 0);
    graphics.lineTo(size, size);
    graphics.lineTo(0, size);
    graphics.closePath();
    graphics.stroke({ color: 0xffffff, width: 2, alpha: 0.1 });

    grid.addChild(graphics);
  }

  setPlayerID(id) {
    this.playerID = id;
  }

  processStateUpdate(entities, metrics) {
    // Calculate network TPS
    const now = performance.now();
    const dt = (now - this.lastUpdateTimestamp) / 1000;
    this.lastUpdateTimestamp = now;

    if (dt > 0) {
      const instantTPS = 1 / dt;
      this.currentTPS = (this.currentTPS * this.tpsFilter) + (instantTPS * (1 - this.tpsFilter));
    }

    this.entityManager.updateEntities(entities);

    // Follow player tank
    const playerTank = this.entityManager.getEntity(this.playerID);
    if (playerTank) {
      this.camera.setTarget(playerTank.position);
    }

    if (metrics) {
      this.updateMetricsUI(metrics);
    }

    // Update HUD (scores, leaderboard)
    this.game.ui.updateHUD(entities, metrics);
  }

  updateMetricsUI(metrics) {
    const el = document.getElementById("debug-metrics");
    if (!el) return;
    
    const count = metrics.entity_count !== undefined ? metrics.entity_count : metrics.EntityCount;
    const tps = Math.round(this.currentTPS);
    
    el.innerText = `TPS: ${tps} | Entities: ${count}`;
  }

  update(deltaTime) {
    this.camera.update(deltaTime);
    this.entityManager.update(deltaTime);

    // Update debug coordinates UI
    const playerTank = this.entityManager.getEntity(this.playerID);
    if (playerTank) {
      const debugEl = document.getElementById("debug-coords");
      if (debugEl) {
        const x = Math.round(playerTank.position.x);
        const y = Math.round(playerTank.position.y);
        debugEl.innerText = `${x}, ${y}`;
      }
    }

    // Apply camera transform to worldContainer
    this.worldContainer.position.set(this.camera.x, this.camera.y);
    this.worldContainer.scale.set(this.camera.zoom);
  }

  screenToWorld(x, y) {
    return {
      x: (x - this.camera.x) / this.camera.zoom,
      y: (y - this.camera.y) / this.camera.zoom,
    };
  }
}
