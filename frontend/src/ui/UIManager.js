export class UIManager {
  constructor(game) {
    this.game = game;
    this.container = document.getElementById("ui-layer");
    this.currentScreen = null;
  }

  onStateChange(state) {
    this.clear();
    switch (state) {
      case "MENU":
        this.showMenu();
        break;
      case "CONNECTING":
        this.showConnecting();
        break;
      case "PLAYING":
        this.showHUD();
        break;
      case "ERROR":
        // Handled separately or as a screen
        break;
    }
  }

  clear() {
    this.container.innerHTML = "";
  }

  showMenu() {
    const menu = document.createElement("div");
    menu.className = "screen menu-screen";
    menu.innerHTML = `
            <div class="menu-content glass">
                <h1 class="logo">zhuch</h1>
                <p class="subtitle">----------------------</p>
                <div class="input-group">
                    <input type="text" id="player-name" placeholder="Enter name..." maxlength="16" value="Player">
                </div>
                <div class="input-group">
                    <select id="room-id">
                        <option value="default">Default Server (US)</option>
                        <option value="europe">Europe Central</option>
                        <option value="asia">Asia East</option>
                    </select>
                </div>
                <button id="start-btn" class="button button-primary">Join Game</button>
            </div>
        `;
    this.container.appendChild(menu);

    const startBtn = document.getElementById("start-btn");
    const nameInput = document.getElementById("player-name");
    const roomSelect = document.getElementById("room-id");

    startBtn.onclick = () => {
      const name = nameInput.value.trim() || "Player";
      const room = roomSelect.value;
      this.game.connect(name, room);
    };
  }

  showConnecting() {
    const screen = document.createElement("div");
    screen.className = "screen connecting-screen";
    screen.innerHTML = `
            <div class="loader-content">
                <div class="spinner"></div>
                <p>Connecting to server...</p>
            </div>
        `;
    this.container.appendChild(screen);
  }

  showHUD() {
    const hud = document.createElement("div");
    hud.className = "hud";
    hud.innerHTML = `
            <div class="hud-top-right">
                <div class="leaderboard glass" id="leaderboard">
                    <h3>Leaderboard</h3>
                    <div id="leaderboard-list"></div>
                </div>
                <button id="settings-btn" class="button button-icon glass" style="margin-top: 12px; width: 100%;">
                    Settings
                </button>
            </div>
            <div id="settings-modal" class="modal glass" style="display: none;">
                <div class="modal-content">
                    <h2>Settings</h2>
                    <div class="setting-item">
                        <label>Graphics Quality</label>
                        <select>
                            <option>Premium</option>
                            <option>Balanced</option>
                            <option>Performance</option>
                        </select>
                    </div>
                    <button class="button button-primary" id="close-settings">Close</button>
                </div>
            </div>
            <div class="hud-bottom-center">
                <div class="debug-panel">
                    <div class="debug-item" id="debug-coords">0, 0</div>
                    <div class="debug-item" id="debug-metrics">Tick: 0ms | Entities: 0</div>
                </div>
                <div class="stats-bar glass">
                    <div class="stat"><span class="label">SCORE</span> <span id="stat-score">0</span></div>
                    <div class="stat"><span class="label">KILLS</span> <span id="stat-kills">0</span></div>
                </div>
            </div>
        `;
    this.container.appendChild(hud);

    const settingsBtn = document.getElementById("settings-btn");
    const settingsModal = document.getElementById("settings-modal");
    const closeSettings = document.getElementById("close-settings");

    settingsBtn.onclick = () => {
      settingsModal.style.display = "block";
    };

    closeSettings.onclick = () => {
      settingsModal.style.display = "none";
    };
  }

  showError(message) {
    const error = document.createElement("div");
    error.className = "error-toast glass";

    // If we're in menu or connecting, it's likely a non-fatal validation error
    const isFatal = this.game.state === "ERROR";

    error.innerHTML = `
      <p>${message}</p>
      ${isFatal ? '<button onclick="window.location.reload()" class="button">Reload</button>' : ""}
    `;
    this.container.appendChild(error);

    if (!isFatal) {
      setTimeout(() => {
        error.style.opacity = "0";
        setTimeout(() => error.remove(), 500);
      }, 3000);
    }
  }

  updateHUD(entities, metrics) {
    // Update score, kills
    const playerTank = entities.find(
      (e) => (e.id || e.ID) === this.game.renderer.playerID,
    );
    if (playerTank) {
      const score =
        playerTank.score !== undefined ? playerTank.score : playerTank.Score;
      const kills =
        playerTank.kills !== undefined ? playerTank.kills : playerTank.Kills;
      document.getElementById("stat-score").innerText = Math.floor(score || 0);
      document.getElementById("stat-kills").innerText = kills || 0;
    }

    // Leaderboard
    const leaderboardList = document.getElementById("leaderboard-list");
    if (leaderboardList) {
      const tanks = entities
        .filter((e) => e.score !== undefined || e.Score !== undefined)
        .sort((a, b) => {
          const sA = a.score !== undefined ? a.score : a.Score;
          const sB = b.score !== undefined ? b.score : b.Score;
          return (sB || 0) - (sA || 0);
        })
        .slice(0, 5);

      leaderboardList.innerHTML = tanks
        .map((t, i) => {
          const id = t.id || t.ID;
          const score = t.score !== undefined ? t.score : t.Score;
          const name = t.name || t.Name || "Tank";
          return `
          <div class="leaderboard-item ${id === this.game.renderer.playerID ? "self" : ""}">
            <span>${i + 1}. ${name}</span>
            <span>${Math.floor(score || 0)}</span>
          </div>
        `;
        })
        .join("");
    }
  }
}
