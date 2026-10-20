namespace Moonwell
{
    /// <summary>Whether a player slot is in the game: empty, playing, or left.</summary>
    public enum SlotState
    {
        /// <summary>No one is in the slot.</summary>
        Empty,

        /// <summary>A person or a computer plays in the slot.</summary>
        Playing,

        /// <summary>The slot's player has left the game.</summary>
        Left,
    }
}
