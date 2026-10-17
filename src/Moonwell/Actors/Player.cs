namespace Moonwell
{
    /// <summary>A player slot of the map: an actor that holds the components of one player, with one object per slot.</summary>
    public sealed class Player : Actor
    {
        /// <summary>The number of player slots, the game's bj_MAX_PLAYER_SLOTS.</summary>
        public const int MaxSlots = 28;

        /// <summary>The player object of each slot, made when the slot is first asked for; null until then.</summary>
        private static readonly Player?[] bySlot = new Player?[MaxSlots];

        /// <summary>Makes the object of a slot, with the game's player for it.</summary>
        private Player(int slot, player handle)
        {
            Slot = slot;
            Handle = handle;
        }

        /// <summary>The slot of the player.</summary>
        public int Slot { get; }

        /// <summary>The game's player in this slot.</summary>
        public player Handle { get; }

        /// <summary>The player object of a slot; made on first ask.</summary>
        public static Player Get(int slot)
        {
            var existing = bySlot[slot];
            if (existing != null)
            {
                return existing;
            }

            var made = new Player(slot, Runtime.World.PlayerAt(slot));
            bySlot[slot] = made;
            return made;
        }

        /// <summary>The player object of a game player; null for null.</summary>
        public static Player? Of(player? handle)
        {
            if (handle == null)
            {
                return null;
            }

            return Get(Runtime.World.SlotOf(handle));
        }

        /// <summary>The player object of a slot when one has been made; null otherwise.</summary>
        internal static Player? Existing(int slot)
        {
            return bySlot[slot];
        }

        /// <summary>Adds a component to the player, and returns it.</summary>
        public T Add<T>(T component) where T : PlayerComponent => AddComponent(component);

        /// <summary>Forgets every player object; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            for (var i = 0; i < MaxSlots; i++)
            {
                bySlot[i] = null;
            }
        }
    }
}
