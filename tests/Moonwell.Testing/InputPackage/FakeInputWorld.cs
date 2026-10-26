using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the input package's cases run against: mouse events a case sends by hand, the local reads a case sets, and a record of the changes the package made for a player's screen.</summary>
    public sealed class FakeInputWorld : IInputWorld
    {
        /// <summary>The three actions the package gave for one player's mouse.</summary>
        public sealed class Listened
        {
            /// <summary>The player the package listened for.</summary>
            public player Player = new player();

            /// <summary>What the package runs on a move.</summary>
            public Action? Moved;

            /// <summary>What the package runs on a press.</summary>
            public Action? Pressed;

            /// <summary>What the package runs on a release.</summary>
            public Action? Released;
        }

        /// <summary>Every ListenMouse call, in order.</summary>
        public List<Listened> Listens = new List<Listened>();

        /// <summary>The world's maximum x, as WorldTopRight gives it.</summary>
        public float MaxX = 4096f;

        /// <summary>The world's maximum y, as WorldTopRight gives it.</summary>
        public float MaxY = 4096f;

        /// <summary>How many times WorldTopRight was called.</summary>
        public int WorldReads;

        /// <summary>The cursor's screen x on this machine.</summary>
        public int ScreenX;

        /// <summary>The cursor's screen y on this machine.</summary>
        public int ScreenY;

        /// <summary>The unit under the cursor on this machine; null for none.</summary>
        public unit? Focus;

        /// <summary>Which buttons (1 to 3) are held on this machine now.</summary>
        public bool[] Held = new bool[4];

        /// <summary>The changes for a screen, as "screen slot x y" and "cursor slot show" or "cursor slot hide", in order.</summary>
        public List<string> Calls = new List<string>();

        /// <summary>The player of the event being delivered.</summary>
        private player? eventPlayer;

        /// <summary>The x of the event being delivered.</summary>
        private float eventX;

        /// <summary>The y of the event being delivered.</summary>
        private float eventY;

        /// <summary>The button of the event being delivered.</summary>
        private int eventButton;

        /// <summary>Sends a move event for a player's mouse; the player must have been listened for.</summary>
        public void Move(Player player, float x, float y)
        {
            Send(player, x, y, 0, 0);
        }

        /// <summary>Sends a press event for a player's mouse.</summary>
        public void Press(Player player, MouseButton button, float x, float y)
        {
            Send(player, x, y, (int)button, 1);
        }

        /// <summary>Sends a release event for a player's mouse.</summary>
        public void Release(Player player, MouseButton button, float x, float y)
        {
            Send(player, x, y, (int)button, 2);
        }

        /// <summary>Runs the action of a kind (0 move, 1 press, 2 release) with the event's values set, and puts the previous ones back.</summary>
        private void Send(Player player, float x, float y, int button, int kind)
        {
            Listened? found = null;
            for (var i = 0; i < Listens.Count; i++)
            {
                if (Listens[i].Player == player.Handle)
                {
                    found = Listens[i];
                }
            }

            if (found == null)
            {
                throw new ExpectationFailed("The package did not listen for slot " + player.Slot + ".");
            }

            var beforePlayer = eventPlayer;
            var beforeX = eventX;
            var beforeY = eventY;
            var beforeButton = eventButton;
            eventPlayer = player.Handle;
            eventX = x;
            eventY = y;
            eventButton = button;
            try
            {
                var action = kind == 0 ? found.Moved : kind == 1 ? found.Pressed : found.Released;
                if (action != null)
                {
                    action();
                }
            }
            finally
            {
                eventPlayer = beforePlayer;
                eventX = beforeX;
                eventY = beforeY;
                eventButton = beforeButton;
            }
        }

        /// <summary>Remembers what to run for a player's mouse.</summary>
        public void ListenMouse(player p, Action moved, Action pressed, Action released)
        {
            var one = new Listened();
            one.Player = p;
            one.Moved = moved;
            one.Pressed = pressed;
            one.Released = released;
            Listens.Add(one);
        }

        /// <summary>The player of the event being delivered.</summary>
        public player EventPlayer() => eventPlayer!;

        /// <summary>The x of the event being delivered.</summary>
        public float EventMouseX() => eventX;

        /// <summary>The y of the event being delivered.</summary>
        public float EventMouseY() => eventY;

        /// <summary>The button of the event being delivered.</summary>
        public int EventMouseButton() => eventButton;

        /// <summary>The world's top-right corner a case set.</summary>
        public void WorldTopRight(out float maxX, out float maxY)
        {
            WorldReads++;
            maxX = MaxX;
            maxY = MaxY;
        }

        /// <summary>The screen x a case set.</summary>
        public int MouseScreenX() => ScreenX;

        /// <summary>The screen y a case set.</summary>
        public int MouseScreenY() => ScreenY;

        /// <summary>The unit a case put under the cursor.</summary>
        public unit? MouseFocusUnit() => Focus;

        /// <summary>Whether a case holds the button.</summary>
        public bool IsMouseButtonPressed(int button) => Held[button];

        /// <summary>Remembers the cursor move.</summary>
        public void SetMouseScreen(player p, int x, int y)
        {
            Calls.Add("screen " + Player.Of(p)!.Slot + " " + x + " " + y);
        }

        /// <summary>Remembers the cursor change.</summary>
        public void ShowCursor(player p, bool show)
        {
            Calls.Add("cursor " + Player.Of(p)!.Slot + " " + (show ? "show" : "hide"));
        }
    }
}
