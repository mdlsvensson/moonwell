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

        /// <summary>Runs an action on the machine of a player; on the other machines it does nothing. A unit's, hero's or item's getter read inside it gives the game's value but does not become the last value read that the object gives once the game has removed it. An error the action throws is reported on that machine and goes no further, so the code after the block runs on every machine and the game stays the same on each.</summary>
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
                catch (Exception error)
                {
                    // The report only prints and logs on this machine.
                    Errors.Report("Local.For", error);
                }
                finally
                {
                    Depth--;
                }
            }
        }
    }
}
