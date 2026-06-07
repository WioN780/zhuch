import { Graphics } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Food extends EntityBase {
  constructor(id, manager) {
    super(id, manager);
    this.graphics = new Graphics();
    this.container.addChild(this.graphics);
    this.type = "square";
    this.size = 10;
    this.rotationSpeed = (Math.random() - 0.5) * 0.05;
    this.draw();
  }

  draw() {
    this.graphics.clear();

    const colors = {
      square: 0x4fc3f7, // Light Blue
      triangle: 0xff8a65, // Deep Orange
      pentagon: 0x9575cd, // Deep Purple
    };

    const color = colors[this.type] || 0xffffff;

    switch (this.type) {
      case "square":
        this.graphics.rect(
          -this.size / 2,
          -this.size / 2,
          this.size,
          this.size,
        );
        break;
      case "triangle":
        this.graphics.poly([
          0,
          -this.size,
          this.size,
          this.size,
          -this.size,
          this.size,
        ]);
        break;
      case "pentagon":
        const sides = 5;
        const step = (Math.PI * 2) / sides;
        const points = [];
        for (let i = 0; i < sides; i++) {
          points.push(Math.sin(i * step) * this.size);
          points.push(Math.cos(i * step) * this.size);
        }
        this.graphics.poly(points);
        break;
    }

    this.graphics.fill({ color: color, alpha: 0.8 });
    this.graphics.stroke({ color: 0xffffff, width: 1, alpha: 0.5 });
  }

  updateData(data) {
    super.updateData(data);
    let redraw = false;

    const type = data.type || data.Type;
    if (type && this.type !== type) {
      this.type = type;
      redraw = true;
    }

    const object = data.object || data.Object;
    if (object) {
      const newSize =
        object.Size ||
        object.size ||
        object.SideLength ||
        object.side_length ||
        15;
      if (this.size !== newSize) {
        this.size = newSize;
        redraw = true;
      }
    }

    if (redraw) this.draw();
  }

  syncRotation() {
    // Food rotation is purely client-side in this game,
    // so we don't want to sync it from the server's default 0.
  }

  update(deltaTime) {
    super.update(deltaTime);
    this.container.rotation += this.rotationSpeed * deltaTime;
  }
}
