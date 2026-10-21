using System;

namespace Moonwell
{
    /// <summary>
    /// Code that runs on one player's machine only, such as what that player sees or hears. Inside it the analyzer
    /// rejects anything that changes the game, which would desync the other machines.
    /// </summary>
    public static class Local
    {
        /// <summary>How many Local.For actions are running now. While it is above 0, getters give the game's value but keep no last value, which would differ between machines once the object is gone.</summary>
        internal static int Depth;

        /// <summary>Runs an action on the machine of a player; on the other machines it does nothing. A unit's, hero's or item's getter read inside it gives the game's value but does not become the last value read that the object gives once the game has removed it.</summary>
        [LocalSafe]
        public static void For(Player player, Action action)
        {
            if (Runtime.World.IsLocal(player.Handle))
            {
                Depth++;
                try
                {
                    action();
                }
                finally
                {
                    Depth--;
                }
            }
        }
    }
}
