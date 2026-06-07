import { Graphics, Container } from "pixi.js";

export class EffectsManager {
  constructor(renderer) {
    this.renderer = renderer;
    this.container = renderer.effectsLayer;
    this.particles = [];
  }

  // Create an explosion at a specific point
  spawnExplosion(x, y, color = 0xffffff, count = 10) {
    for (let i = 0; i < count; i++) {
      this.spawnParticle(x, y, color);
    }
  }

  // Spawn a single particle
  spawnParticle(x, y, color) {
    const graphics = new Graphics();
    const size = 2 + Math.random() * 4;

    graphics.rect(-size / 2, -size / 2, size, size);
    graphics.fill({ color: color, alpha: 0.8 });

    const angle = Math.random() * Math.PI * 2;
    const speed = 2 + Math.random() * 5;

    const particle = {
      graphics,
      x,
      y,
      vx: Math.cos(angle) * speed,
      vy: Math.sin(angle) * speed,
      life: 1.0,
      decay: 0.02 + Math.random() * 0.05,
    };

    this.container.addChild(graphics);
    this.particles.push(particle);
  }

  spawnTrailParticle(x, y, color = 0xffffff) {
    const graphics = new Graphics();
    const size = 1 + Math.random() * 2;

    graphics.rect(-size / 2, -size / 2, size, size);
    graphics.fill({ color: color, alpha: 0.3 });

    const particle = {
      graphics,
      x,
      y,
      vx: (Math.random() - 0.5) * 0.5,
      vy: (Math.random() - 0.5) * 0.5,
      life: 0.5,
      decay: 0.05,
    };

    this.container.addChild(graphics);
    this.particles.push(particle);
  }

  update(deltaTime) {
    for (let i = this.particles.length - 1; i >= 0; i--) {
      const p = this.particles[i];
      p.life -= p.decay * deltaTime;

      if (p.life <= 0) {
        this.container.removeChild(p.graphics);
        p.graphics.destroy();
        this.particles.splice(i, 1);
        continue;
      }

      p.x += p.vx * deltaTime;
      p.y += p.vy * deltaTime;

      p.graphics.position.set(p.x, p.y);
      p.graphics.alpha = p.life;
      p.graphics.scale.set(p.life);
    }
  }
}
