using System;

namespace Moonwell
{
    /// <summary>
    /// Code that runs on one player's machine only, such as what that player sees or hears. Inside it the analyzer
    /// rejects anything that changes the game, which would desync the other machines.
    /// </summary>
    public static class Local
    {
        /// <summary>Runs an action on the machine of a player; on the other machines it does nothing.</summary>
        [LocalSafe]
        public static void For(Player player, Action action)
        {
            if (Runtime.World.IsLocal(player.Handle))
            {
                action();
            }
        }
    }
}
