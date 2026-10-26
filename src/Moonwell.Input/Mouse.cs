namespace Moonwell
{
    /// <summary>
    /// A player's mouse: the last place in the world its synced events gave, and the buttons they say are down. Get one
    /// with <c>player.Mouse()</c>. The position and the buttons are the same on every machine, so a map reads them
    /// anywhere; the cursor's place on the screen, the unit under it and a button held now are this machine's, and are
    /// read inside <c>Local.For</c>.
    /// </summary>
    public sealed class Mouse
    {
        /// <summary>The buttons the events said are down, indexed by MouseButton (1 to 3).</summary>
        private readonly bool[] down = new bool[4];

        /// <summary>Makes the mouse of a slot.</summary>
        internal Mouse(int slot)
        {
            Slot = slot;
        }

        /// <summary>The slot of the mouse's player.</summary>
        internal int Slot { get; }

        /// <summary>The player of this mouse.</summary>
        [LocalSafe]
        public Player Player => Player.Get(Slot);

        /// <summary>
        /// The x in the world the last mouse event gave, or 0 before one did. Events that read exactly (0, 0), or exactly
        /// the world's top-right corner less 32 on both (what the game gave at times past the map's edge), are not a place
        /// and keep the last position, so the map's exact centre and that corner cannot be read. It does not follow the
        /// camera: the world moves under a still cursor and no event comes. A press over the interface (command card,
        /// minimap, portrait) gives the ground under it. Online, events arrive after the network delay.
        /// </summary>
        [LocalSafe]
        public float X { get; private set; }

        /// <summary>The y in the world the last mouse event gave; see <see cref="X"/> for what it keeps and what it does not follow.</summary>
        [LocalSafe]
        public float Y { get; private set; }

        /// <summary>
        /// The last place in the world a mouse event gave, as a new Vector2 each time it is read (read X and Y in code
        /// that runs often). Zero before the first event over the world. It keeps its last position on a reading of
        /// (0, 0) or of the world's top-right corner less 32, and does not follow a camera move. A press over the
        /// interface (command card, minimap, portrait) gives the ground under it. Online, events arrive after the
        /// network delay.
        /// </summary>
        [LocalSafe]
        public Vector2 Position => new Vector2(X, Y);

        /// <summary>Whether an event has given a position: false until the first event over the world.</summary>
        [LocalSafe]
        public bool HasPosition { get; private set; }

        /// <summary>Whether the events say a button is down: kept from the press and release events, so online it is the network delay behind the physical button.</summary>
        [LocalSafe]
        public bool IsDown(MouseButton button)
        {
            return down[(int)button];
        }

        /// <summary>The cursor's x on this machine's screen, in pixels. This machine's alone, so it is read inside Local.For.</summary>
        public int ScreenX
        {
            [MachineLocal]
            get { return InputSystem.World.MouseScreenX(); }
        }

        /// <summary>The cursor's y on this machine's screen, in pixels. This machine's alone, so it is read inside Local.For.</summary>
        public int ScreenY
        {
            [MachineLocal]
            get { return InputSystem.World.MouseScreenY(); }
        }

        /// <summary>The unit under the cursor on this machine now, or null. This machine's alone, so it is read inside Local.For.</summary>
        public Unit? FocusUnit
        {
            [MachineLocal]
            get { return Unit.Of(InputSystem.World.MouseFocusUnit()); }
        }

        /// <summary>Whether a button is held on this machine now, with no network delay (IsDown has it). This machine's alone, so it is read inside Local.For.</summary>
        [MachineLocal]
        public bool IsHeldNow(MouseButton button)
        {
            return InputSystem.World.IsMouseButtonPressed((int)button);
        }

        /// <summary>Puts the player's cursor at a place on the screen, in pixels. It moves only that player's cursor, so it is safe anywhere.</summary>
        [LocalSafe]
        public void SetScreenPosition(int x, int y)
        {
            InputSystem.World.SetMouseScreen(Player.Handle, x, y);
        }

        /// <summary>Shows or hides the player's cursor. It changes only what that player sees, so it is safe anywhere.</summary>
        [LocalSafe]
        public void ShowCursor(bool show)
        {
            InputSystem.World.ShowCursor(Player.Handle, show);
        }

        /// <summary>Takes the place an event gave, unless it is (0, 0) or the corner: those readings are not a place.</summary>
        internal void Take(float x, float y, float cornerX, float cornerY)
        {
            if (x == 0f && y == 0f)
            {
                return;
            }

            if (x == cornerX && y == cornerY)
            {
                return;
            }

            X = x;
            Y = y;
            HasPosition = true;
        }

        /// <summary>Sets whether the events say a button is down.</summary>
        internal void SetDown(MouseButton button, bool isDown)
        {
            down[(int)button] = isDown;
        }
    }
}
