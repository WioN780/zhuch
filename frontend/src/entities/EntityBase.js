import { Container, Graphics } from "pixi.js";

export class EntityBase {
  constructor(id) {
    this.id = id;
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
    if (data.Object) {
      this.targetPosition = {
        x: data.Object.Center.X,
        y: data.Object.Center.Y,
      };

      // Snap to position if this is the first update to prevent "flying in" from (0,0)
      if (!this.initialized) {
        this.position.x = this.targetPosition.x;
        this.position.y = this.targetPosition.y;
        this.initialized = true;
      }
    }
    if (data.Vel) {
      this.targetVelocity = { x: data.Vel.X, y: data.Vel.Y };
    }

    this.health = data.Health;
    this.maxHealth = data.MaxHealth;
    this.updateHealthBar();
  }

  updateHealthBar() {
    if (!this.health || this.health >= this.maxHealth) {
      this.healthBar.visible = false;
      return;
    }

    this.healthBar.visible = true;
    const width = 40;
    const height = 4;
    const pct = Math.max(0, this.health / this.maxHealth);

    // Offset below the entity
    const yOffset = 35;

    this.healthBarBg.clear();
    this.healthBarBg.rect(-width / 2, yOffset, width, height);
    this.healthBarBg.fill({ color: 0x000000, alpha: 0.5 });

    this.healthBarFill.clear();
    this.healthBarFill.rect(-width / 2, yOffset, width * pct, height);
    this.healthBarFill.fill({ color: 0x81c784 }); // green
  }

  update(deltaTime) {
    if (!this.initialized) return;
    this.position.x +=
      (this.targetPosition.x - this.position.x) * this.lerpFactor * deltaTime;
    this.position.y +=
      (this.targetPosition.y - this.position.y) * this.lerpFactor * deltaTime;

    // Update container position
    this.container.position.set(this.position.x, this.position.y);
  }

  destroy() {
    if (this.container.parent) {
      this.container.parent.removeChild(this.container);
    }
    this.container.destroy({ children: true });
  }
}
