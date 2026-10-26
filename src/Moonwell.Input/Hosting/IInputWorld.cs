using System;

namespace Moonwell.Hosting
{
    /// <summary>The game as the input package sees it: the synced mouse events, and the machine-local reads and the changes for one player's screen. Types are the game's numbers.</summary>
    public interface IInputWorld
    {
        /// <summary>Starts delivering one player's mouse events: the three actions run when the mouse moves, a button goes down and a button comes up. Inside them, <c>EventPlayer</c>, <c>EventMouseX</c>, <c>EventMouseY</c> and <c>EventMouseButton</c> are the event's.</summary>
        void ListenMouse(player p, Action moved, Action pressed, Action released);

        /// <summary>The player of the mouse event being delivered.</summary>
        player EventPlayer();

        /// <summary>The x in the world the mouse event gave.</summary>
        float EventMouseX();

        /// <summary>The y in the world the mouse event gave.</summary>
        float EventMouseY();

        /// <summary>The button of the press or release event being delivered: 1 for left, 2 for middle, 3 for right, 0 for any other.</summary>
        int EventMouseButton();

        /// <summary>The top-right corner of the world's bounds.</summary>
        void WorldTopRight(out float maxX, out float maxY);

        /// <summary>The cursor's x on this machine's screen, in pixels.</summary>
        [MachineLocal]
        int MouseScreenX();

        /// <summary>The cursor's y on this machine's screen, in pixels.</summary>
        [MachineLocal]
        int MouseScreenY();

        /// <summary>The unit under the cursor on this machine now; null when none.</summary>
        [MachineLocal]
        unit? MouseFocusUnit();

        /// <summary>Whether a button (1 left, 2 middle, 3 right) is held on this machine now.</summary>
        [MachineLocal]
        bool IsMouseButtonPressed(int button);

        /// <summary>Puts a player's cursor at a screen position, on that player's machine only.</summary>
        void SetMouseScreen(player p, int x, int y);

        /// <summary>Shows or hides a player's cursor, on that player's machine only.</summary>
        void ShowCursor(player p, bool show);
    }
}
