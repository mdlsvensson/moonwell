using System;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the input package sees it, through the game's natives; the only class of the package that calls them.</summary>
    internal sealed class NativeInputWorld : IInputWorld
    {
        /// <summary>Makes the three triggers of a player's mouse: its moves, its presses and its releases.</summary>
        public void ListenMouse(player p, Action moved, Action pressed, Action released)
        {
            var move = Natives.CreateTrigger();
            Natives.TriggerRegisterPlayerEvent(move, p, Natives.EVENT_PLAYER_MOUSE_MOVE);
            Natives.TriggerAddAction(move, moved);
            var press = Natives.CreateTrigger();
            Natives.TriggerRegisterPlayerEvent(press, p, Natives.EVENT_PLAYER_MOUSE_DOWN);
            Natives.TriggerAddAction(press, pressed);
            var release = Natives.CreateTrigger();
            Natives.TriggerRegisterPlayerEvent(release, p, Natives.EVENT_PLAYER_MOUSE_UP);
            Natives.TriggerAddAction(release, released);
        }

        /// <summary>The player of the event being delivered.</summary>
        public player EventPlayer() => Natives.GetTriggerPlayer();

        /// <summary>The x the event gave.</summary>
        public float EventMouseX() => Natives.BlzGetTriggerPlayerMouseX();

        /// <summary>The y the event gave.</summary>
        public float EventMouseY() => Natives.BlzGetTriggerPlayerMouseY();

        /// <summary>The event's button, compared with the game's three constants: the game's right button was seen with handle id 4, not common.j's 3, so its number is not used.</summary>
        public int EventMouseButton()
        {
            var button = Natives.BlzGetTriggerPlayerMouseButton();
            if (button == Natives.MOUSE_BUTTON_TYPE_LEFT)
            {
                return 1;
            }

            if (button == Natives.MOUSE_BUTTON_TYPE_MIDDLE)
            {
                return 2;
            }

            if (button == Natives.MOUSE_BUTTON_TYPE_RIGHT)
            {
                return 3;
            }

            return 0;
        }

        /// <summary>The world's top-right corner; GetWorldBounds makes a rect, which is read and left.</summary>
        public void WorldTopRight(out float maxX, out float maxY)
        {
            var bounds = Natives.GetWorldBounds();
            maxX = Natives.GetRectMaxX(bounds);
            maxY = Natives.GetRectMaxY(bounds);
        }

        /// <summary>The cursor's x on the screen of this machine.</summary>
        public int MouseScreenX() => Natives.BlzGetMouseScreenPosX();

        /// <summary>The cursor's y on the screen of this machine.</summary>
        public int MouseScreenY() => Natives.BlzGetMouseScreenPosY();

        /// <summary>The unit under the cursor on this machine; null when none.</summary>
        public unit? MouseFocusUnit() => Natives.BlzGetMouseFocusUnit();

        /// <summary>Whether a button is held on this machine now.</summary>
        public bool IsMouseButtonPressed(int button)
        {
            if (button == 1)
            {
                return Natives.BlzIsMouseButtonPressed(Natives.MOUSE_BUTTON_TYPE_LEFT);
            }

            if (button == 2)
            {
                return Natives.BlzIsMouseButtonPressed(Natives.MOUSE_BUTTON_TYPE_MIDDLE);
            }

            return Natives.BlzIsMouseButtonPressed(Natives.MOUSE_BUTTON_TYPE_RIGHT);
        }

        /// <summary>Moves a player's cursor on that player's machine only.</summary>
        public void SetMouseScreen(player p, int x, int y)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.BlzSetMousePos(x, y);
            }
        }

        /// <summary>Shows or hides a player's cursor on that player's machine only.</summary>
        public void ShowCursor(player p, bool show)
        {
            if (Natives.GetLocalPlayer() == p)
            {
                Natives.BlzEnableCursor(show);
            }
        }
    }
}
