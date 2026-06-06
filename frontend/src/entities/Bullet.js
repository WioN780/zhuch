import { Graphics } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Bullet extends EntityBase {
  constructor(id, manager) {
    super(id, manager);
    this.graphics = new Graphics();
    this.container.addChild(this.graphics);
    this.radius = 5;
    this.draw();
  }

  draw() {
    this.graphics.clear();
    this.graphics.circle(0, 0, this.radius);
    this.graphics.fill({ color: 0xffffff, alpha: 0.9 });
  }

  updateData(data) {
    super.updateData(data);
    const object = data.object || data.Object;
    if (object && (object.Radius || object.radius)) {
      const newRadius = object.Radius || object.radius;
      if (this.radius !== newRadius) {
        this.radius = newRadius;
        this.draw();
      }
    }
  }
}
