namespace Moonwell
{
    /// <summary>What a player gives for input.</summary>
    public static class InputExtensions
    {
        /// <summary>The player's mouse: one for each slot, from 0 to 27, made when the package starts. Throws InvalidOperationException before that.</summary>
        [LocalSafe]
        public static Mouse Mouse(this Player player)
        {
            return InputSystem.MouseOf(player.Slot);
        }
    }
}
