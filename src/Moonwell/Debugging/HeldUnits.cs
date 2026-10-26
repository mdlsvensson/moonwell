#if DEBUG
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>
    /// The units a halt holds: it pauses only units not already paused, and lets go only those, as PauseUnit is a flag and
    /// not a count, so a unit the map paused itself stays paused.
    /// </summary>
    internal sealed class HeldUnits
    {
        /// <summary>The debug world that pauses units.</summary>
        private readonly IDebugWorld world;

        /// <summary>The units this has paused, in the order they were held.</summary>
        private readonly List<Unit> held = new List<Unit>();

        /// <summary>Makes the held units of a debug world, none held.</summary>
        internal HeldUnits(IDebugWorld world)
        {
            this.world = world;
        }

        /// <summary>Pauses every live unit not already paused, and remembers those.</summary>
        internal void Hold()
        {
            var live = KnownUnits.Live;
            for (var i = 0; i < live.Count; i++)
            {
                Add(live[i]);
            }
        }

        /// <summary>Pauses one unit when it is live and not already paused, and remembers it.</summary>
        internal void Add(Unit unit)
        {
            if (unit.HasEnded || unit.IsGone || world.IsPaused(unit.Handle))
            {
                return;
            }

            world.Pause(unit.Handle, true);
            held.Add(unit);
        }

        /// <summary>Lets go of the units this paused that are still in the game, and forgets them all.</summary>
        internal void LetGo()
        {
            for (var i = 0; i < held.Count; i++)
            {
                var unit = held[i];
                if (!unit.HasEnded && !unit.IsGone)
                {
                    world.Pause(unit.Handle, false);
                }
            }

            held.Clear();
        }
    }
}
#endif
