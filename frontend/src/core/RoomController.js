export class RoomController {
    constructor(game) {
        this.game = game;
    }

    /**
     * Get the base URL based on the user's selection
     */
    getBaseURL(customURL = null) {
        let url = customURL || "zhuch-production.up.railway.app";
        if (!url.startsWith("http")) {
            url = url.includes("localhost") ? `http://${url}` : `https://${url}`;
        }
        return url;
    }

    /**
     * API: Fetch all active rooms from a server
     */
    async fetchRooms(customURL = null) {
        const baseURL = this.getBaseURL(customURL);
        try {
            const response = await fetch(`${baseURL}/rooms`);
            if (!response.ok) throw new Error("Failed to fetch rooms");
            return await response.json();
        } catch (err) {
            console.error("RoomController Error:", err);
            return [];
        }
    }

    /**
     * API: Create a new room
     */
    async createRoom(roomID, config, customURL = null) {
        const baseURL = this.getBaseURL(customURL);
        const response = await fetch(`${baseURL}/create`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ id: roomID, config })
        });

        if (!response.ok) {
            const msg = await response.text();
            throw new Error(msg);
        }
        return true;
    }

    /**
     * Action: Join a game
     */
    async joinGame(playerName, roomID, customURL = null) {
        try {
            this.game.setState("CONNECTING");
            await this.game.socket.connect(playerName, roomID, customURL);
            this.game.setState("PLAYING");
        } catch (err) {
            console.error("Join failed:", err);
            this.game.setState("ERROR");
            this.game.ui.showError(`Connection failed: ${err.message}`);
        }
    }
}
