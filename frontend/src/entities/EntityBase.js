import { Container, Graphics } from "pixi.js";

export class EntityBase {
  constructor(id, manager) {
    this.id = id;
    this.manager = manager;
    this.container = new Container();

    this.position = { x: 0, y: 0 };
    this.velocity = { x: 0, y: 0 };
    this.rotation = 0;

    this.stateBuffer = [];
    this.interpolationDelay = 100; // ms behind server

    this.initialized = false;
    this.lastHealthPct = -1;

    this.healthBar = new Container();
    this.healthBarBg = new Graphics();
    this.healthBarFill = new Graphics();
    this.healthBar.addChild(this.healthBarBg);
    this.healthBar.addChild(this.healthBarFill);
    this.container.addChild(this.healthBar);
    this.healthBar.visible = false;
  }

  get isLocal() {
    return this.id === this.manager?.renderer?.playerID;
  }

  updateData(data, timestamp = performance.now()) {
    const object = data.object || data.Object;
    const vel = data.vel || data.Vel;
    const center = object?.Center || object?.center || { X: 0, Y: 0 };
    const rotation =
      data.orientation !== undefined ? data.orientation : data.Orientation || 0;

    const newState = {
      timestamp,
      pos: { x: center.X || center.x, y: center.Y || center.y },
      vel: { x: vel?.X || vel?.x || 0, y: vel?.Y || vel?.y || 0 },
      rotation: rotation,
      health: data.health !== undefined ? data.health : data.Health,
      maxHealth:
        data.max_health !== undefined ? data.max_health : data.MaxHealth || 100,
      name: data.name || data.Name,
      ownerID: data.owner_id || data.OwnerID,
    };

    if (!this.initialized) {
      this.position = { ...newState.pos };
      this.velocity = { ...newState.vel };
      this.rotation = newState.rotation;
      this.initialized = true;
    }

    if (this.isLocal) {
      this.onServerUpdate(newState);
    } else {
      this.stateBuffer.push(newState);
      if (this.stateBuffer.length > 20) this.stateBuffer.shift();
    }

    this.health = newState.health;
    this.maxHealth = newState.maxHealth;
    this.updateHealthBar();
  }

  onServerUpdate(serverState) {}

  update(deltaTime, deltaMS) {
    if (!this.initialized) return;

    if (this.isLocal) {
      this.updateLocal(deltaTime, deltaMS);
    } else {
      this.updateInterpolation();
    }

    // Only apply default position if NOT local
    if (!this.isLocal) {
      this.syncContainer();
    }
  }

  syncContainer() {
    this.container.position.set(this.position.x, this.position.y);
  }

  updateInterpolation() {
    if (this.stateBuffer.length < 1) return;

    const isLocalBullet =
      this.stateBuffer[this.stateBuffer.length - 1].ownerID ===
      this.manager.renderer.playerID;

    if (isLocalBullet) {
      const latest = this.stateBuffer[this.stateBuffer.length - 1];
      this.position = { ...latest.pos };
      this.rotation = latest.rotation;
      return;
    }

    if (this.stateBuffer.length < 2) return;

    const renderTime = performance.now() - this.interpolationDelay;
    let i = 0;
    for (; i < this.stateBuffer.length - 1; i++) {
      if (this.stateBuffer[i + 1].timestamp > renderTime) break;
    }

    const stateA = this.stateBuffer[i];
    const stateB = this.stateBuffer[i + 1];

    if (
      stateA &&
      stateB &&
      renderTime >= stateA.timestamp &&
      renderTime <= stateB.timestamp
    ) {
      const lerpFactor =
        (renderTime - stateA.timestamp) / (stateB.timestamp - stateA.timestamp);
      this.position.x =
        stateA.pos.x + (stateB.pos.x - stateA.pos.x) * lerpFactor;
      this.position.y =
        stateA.pos.y + (stateB.pos.y - stateA.pos.y) * lerpFactor;

      let diff = stateB.rotation - stateA.rotation;
      while (diff > Math.PI) diff -= Math.PI * 2;
      while (diff < -Math.PI) diff += Math.PI * 2;
      this.rotation = stateA.rotation + diff * lerpFactor;
    } else if (stateB && renderTime > stateB.timestamp) {
      this.position = { ...stateB.pos };
      this.rotation = stateB.rotation;
    }
  }

  updateLocal(deltaTime, deltaMS) {}

  updateHealthBar() {
    if (this.health === undefined || this.health >= this.maxHealth) {
      this.healthBar.visible = false;
      return;
    }
    const pct = Math.max(0, this.health / this.maxHealth);
    if (this.lastHealthPct === pct) return;
    this.lastHealthPct = pct;
    this.healthBar.visible = true;
    const width = 40;
    const height = 4;
    const yOffset = 35;
    this.healthBarBg
      .clear()
      .rect(-width / 2, yOffset, width, height)
      .fill({ color: 0x000000, alpha: 0.5 });
    this.healthBarFill
      .clear()
      .rect(-width / 2, yOffset, width * pct, height)
      .fill({ color: 0x81c784 });
  }

  reset() {
    this.initialized = false;
    this.stateBuffer = [];
    this.lastHealthPct = -1;
    this.healthBar.visible = false;
  }

  destroy() {
    if (this.container.parent)
      this.container.parent.removeChild(this.container);
    this.container.destroy({ children: true });
  }
}
