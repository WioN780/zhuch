import { Graphics } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Tank extends EntityBase {
  constructor(id) {
    super(id);

    this.body = new Graphics();
    this.barrel = new Graphics();

    this.container.addChild(this.barrel);
    this.container.addChild(this.body);

    this.radius = 20;
    this.barrelAngle = 0;
    this.targetBarrelAngle = 0;

    this.draw();
  }

  draw() {
    // Body circle
    this.body.clear();
    this.body.circle(0, 0, this.radius);
    this.body.fill({ color: 0x1a1a1a });
    this.body.stroke({ color: 0xffffff, width: 1.5, alpha: 0.8 });

    // Barrel rectangle
    this.barrel.clear();
    this.barrel.rect(
      0,
      -this.radius * 0.4,
      this.radius * 1.5,
      this.radius * 0.8,
    );
    this.barrel.fill({ color: 0x1a1a1a });
    this.barrel.stroke({ color: 0xffffff, width: 1.5, alpha: 0.8 });
  }

  updateData(data) {
    super.updateData(data);
    if (data.Object && data.Object.Radius) {
      if (this.radius !== data.Object.Radius) {
        this.radius = data.Object.Radius;
        this.draw();
      }
    }

    if (data.Orientation !== undefined) {
      this.targetBarrelAngle = data.Orientation;
    }
  }

  update(deltaTime) {
    super.update(deltaTime);

    this.barrelAngle = this.targetBarrelAngle;
    this.barrel.rotation = this.barrelAngle;
  }
}
