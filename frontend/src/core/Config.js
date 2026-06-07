export const CONFIG = {
  // World Settings
  WORLD: {
    SIZE: 2000,
    GRID_SPACING: 100,
  },

  // Physics Defaults (Overwritten by server 'init' message)
  PHYSICS: {
    FRICTION: 0.9,
    ACCELERATION: 1.5,
    MAX_SPEED: 15.0,
    WEIGHT: 10.0,
    SERVER_TICK_RATE: 20, // Hz
    SERVER_TICK_MS: 50,   // 1000 / 20
  },

  // Interpolation & Smoothing
  SMOOTHING: {
    TANK_INTERPOLATION_DELAY: 100, // ms
    BULLET_INTERPOLATION_DELAY: 50, // ms
    OFFSET_BLEED: 0.15,            // % per frame
    TPS_FILTER: 0.9,               // Smoothing for TPS display
  },

  // UI & Rendering Defaults
  VISUALS: {
    BACKGROUND_COLOR: 0x0a0a0a,
    TANK_RADIUS: 20,
    BULLET_RADIUS: 5,
    HEALTH_BAR: {
      WIDTH: 40,
      HEIGHT: 4,
      OFFSET_Y: 35,
    }
  }
};
