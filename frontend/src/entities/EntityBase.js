import { Container, Graphics } from "pixi.js";

export class EntityBase {
  constructor(id, manager) {
    this.id = id;
    this.manager = manager;
    this.container = new Container();
    this.position = { x: 0, y: 0 };
    this.velocity = { x: 0, y: 0 };

    this.targetPosition = { x: 0, y: 0 };
    this.targetVelocity = { x: 0, y: 0 };

    this.initialized = false;
    this.lerpFactor = 0.2;

    // Health Bar UI
    this.healthBar = new Container();
    this.healthBarBg = new Graphics();
    this.healthBarFill = new Graphics();
    this.healthBar.addChild(this.healthBarBg);
    this.healthBar.addChild(this.healthBarFill);
    this.container.addChild(this.healthBar);
    this.healthBar.visible = false;
  }

  updateData(data) {
    const object = data.object || data.Object;
    const vel = data.vel || data.Vel;

    if (object) {
      // Handle different field cases (Center vs center)
      const center = object.Center || object.center || { X: 0, Y: 0 };
      this.targetPosition = {
        x: center.X || center.x,
        y: center.Y || center.y,
      };

      if (!this.initialized) {
        this.position.x = this.targetPosition.x;
        this.position.y = this.targetPosition.y;
        this.initialized = true;
      }
    }
    
    if (vel) {
      this.targetVelocity = { x: vel.X || vel.x, y: vel.Y || vel.y };
    }

    const health = data.health !== undefined ? data.health : data.Health;
    const maxHealth = data.max_health !== undefined ? data.max_health : (data.MaxHealth || 100);
    
    if (this.health !== health || this.maxHealth !== maxHealth) {
      this.health = health;
      this.maxHealth = maxHealth;
      this.updateHealthBar();
    }
  }

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

    this.healthBarBg.clear()
      .rect(-width / 2, yOffset, width, height)
      .fill({ color: 0x000000, alpha: 0.5 });

    this.healthBarFill.clear()
      .rect(-width / 2, yOffset, width * pct, height)
      .fill({ color: 0x81c784 });
  }

  update(deltaTime) {
    if (!this.initialized) return;

    // Smoother interpolation using velocity prediction
    const lerp = 1 - Math.pow(0.1, deltaTime / 60); // Independent of frame rate (approx 0.1 at 60fps)
    
    this.position.x += (this.targetPosition.x - this.position.x) * this.lerpFactor * deltaTime;
    this.position.y += (this.targetPosition.y - this.position.y) * this.lerpFactor * deltaTime;

    // Optional: add a bit of velocity to predict next frame
    // this.position.x += this.targetVelocity.x * deltaTime * 0.1;
    // this.position.y += this.targetVelocity.y * deltaTime * 0.1;

    this.container.position.set(this.position.x, this.position.y);
  }

  reset() {
    this.initialized = false;
    this.lastHealthPct = -1;
    this.healthBar.visible = false;
  }

  destroy() {
    if (this.container.parent) {
      this.container.parent.removeChild(this.container);
    }
    this.container.destroy({ children: true });
  }
}
