using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>Runs each hit through its stages: the package's state, and its actions on the game's damage events.</summary>
    internal static class Pipeline
    {
        /// <summary>How long a chain of hits dealt from reactions may be.</summary>
        internal const int MaxChain = 8;

        /// <summary>The world the package talks to; set by Install.</summary>
        internal static IDamageWorld World = null!;

        /// <summary>Whether the package listens for the game's damage events.</summary>
        private static bool listening;

        /// <summary>Whether a class implementing IDamageWatch has been seen.</summary>
        internal static bool Watched;

        /// <summary>Makes a world the package's world, clears its state and plugs it into the runtime.</summary>
        internal static void Install(IDamageWorld world)
        {
            World = world;
            listening = false;
            Watched = false;
            Events.DamageListener = Listen;
            Hooks.Seen = Saw;
        }

        /// <summary>Starts listening for the game's damage events, once.</summary>
        internal static void Listen()
        {
            if (listening)
            {
                return;
            }

            listening = true;
            World.Listen(Damaging, Damaged);
        }

        /// <summary>A class seen for the first time: one implementing a damage interface makes the package listen.</summary>
        private static void Saw(object obj)
        {
            if (obj is IDamageWatch)
            {
                Watched = true;
            }

            if (obj is IDamageSource || obj is IDamageTarget || obj is IDamageWatch)
            {
                Listen();
            }
        }

        /// <summary>The game's damaging event: the before-armor stage.</summary>
        private static void Damaging()
        {
        }

        /// <summary>The game's damaged event: the after-armor and lethal stages.</summary>
        private static void Damaged()
        {
        }
    }
}
