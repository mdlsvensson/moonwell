namespace Moonwell
{
    /// <summary>The bit of each reaction a class can override; a class's mask holds the bits of the ones it overrides.</summary>
    internal static class Hook
    {
        /// <summary>A unit takes damage.</summary>
        internal const int OnDamaged = 1;

        /// <summary>A unit deals damage.</summary>
        internal const int OnDealtDamage = 2;

        /// <summary>A unit dies.</summary>
        internal const int OnDeath = 4;

        /// <summary>A unit kills another.</summary>
        internal const int OnKill = 8;

        /// <summary>A unit casts a spell.</summary>
        internal const int OnCast = 16;

        /// <summary>A unit picks up an item.</summary>
        internal const int OnItemPickedUp = 32;

        /// <summary>A unit drops an item.</summary>
        internal const int OnItemDropped = 64;

        /// <summary>An item is picked up.</summary>
        internal const int OnPickedUp = 128;

        /// <summary>An item is dropped.</summary>
        internal const int OnDropped = 256;

        /// <summary>A unit enters the map.</summary>
        internal const int OnUnitCreated = 512;

        /// <summary>A unit in the map takes damage.</summary>
        internal const int OnUnitDamaged = 1024;

        /// <summary>A unit in the map dies.</summary>
        internal const int OnUnitDied = 2048;

        /// <summary>A unit in the map casts a spell.</summary>
        internal const int OnUnitCast = 4096;

        /// <summary>The game starts.</summary>
        internal const int OnGameStart = 8192;

        /// <summary>A player leaves the game.</summary>
        internal const int OnLeave = 16384;
    }
}
