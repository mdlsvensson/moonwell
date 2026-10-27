using System.Collections.Generic;

namespace Moonwell.Engine
{
    /// <summary>The physics pass: once a step, before the gizmos' ticks, it moves every pushed unit, in the order their pushes began, then every missile, in the order they were made, each meeting the walls right after its move, so a missile's contacts later in the step see each unit where its push left it. The interactions cycle runs it through its hook, guarded.</summary>
    internal static class Pass
    {
        /// <summary>The missiles, in the order they were made; an ended one stays until the pass packs the list. Made on first use after an install. It holds no null, so the pass reads it raw.</summary>
        private static List<Missile>? missiles;

        /// <summary>Whether a missile has ended since the list was last packed.</summary>
        private static bool dropped;

        /// <summary>How many missiles have been made since the install: each is numbered by it, so of two the one made first is known.</summary>
        private static int made;

        /// <summary>Whether the world's bounds have been read since the install.</summary>
        private static bool boundsRead;

        /// <summary>The world's left edge; read once.</summary>
        internal static float minX;

        /// <summary>The world's bottom edge; read once.</summary>
        internal static float minY;

        /// <summary>The world's right edge; read once.</summary>
        internal static float maxX;

        /// <summary>The world's top edge; read once.</summary>
        internal static float maxY;

        /// <summary>Forgets the missiles, the pushes and the bounds; an install starts the pass afresh. trackingKept says the interactions package was not installed afresh since the pushes began, so the count they hold its cycle awake with is still there to lower.</summary>
        public static void Reset(bool trackingKept)
        {
            missiles = null;
            dropped = false;
            made = 0;
            boundsRead = false;
            Pushes.Reset(trackingKept);
            Contacts.Reset();
            Walls.Reset();
        }

        /// <summary>Adds a missile at the end of the list, as it is created, numbered after the one made before it; it moves from the next step.</summary>
        public static void Add(Missile missile)
        {
            made++;
            missile.made = made;
            if (missiles == null)
            {
                missiles = new List<Missile>();
            }

            Raw.Append(missiles, missile);
        }

        /// <summary>Notes that a missile has ended, so the pass packs the list after its step.</summary>
        public static void Dropped()
        {
            dropped = true;
        }

        /// <summary>Moves each pushed unit, then each missile that was in the list when the step began and has not begun to end, in the order they were made, each meeting the walls after its move; then sets the range of the missiles' checks against each other, which covers the farthest any of them moved, and packs away the ended ones. What a missile's reaction makes moves from the next step.</summary>
        public static void Run()
        {
            var world = Physics.World;
            if (!boundsRead)
            {
                float left;
                float bottom;
                float right;
                float top;
                world.WorldBounds(out left, out bottom, out right, out top);
                minX = left;
                minY = bottom;
                maxX = right;
                maxY = top;
                boundsRead = true;
            }

            var step = Interactions.Current().Step;
            Pushes.Run(step);
            var list = missiles;
            if (list == null)
            {
                return;
            }

            var grid = Interactions.Grid;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var missile = Raw.At(list, i);
                if (!missile.ending)
                {
                    Flight.Step(missile, step, grid, world);
                    if (!missile.ending)
                    {
                        Contacts.Reach(missile);
                        if (missile.wallContact != Contact.None && !missile.resting)
                        {
                            Contacts.AgainstWalls(missile);
                        }
                    }
                }
            }

            if (Contacts.meeting)
            {
                Contacts.MeetRanges(list, count);
            }

            if (dropped)
            {
                Pack(list);
            }
        }

        /// <summary>Wakes each resting missile in a box, as the deck there has gone: it falls from its next step.</summary>
        public static void GroundGone(float minX, float minY, float maxX, float maxY)
        {
            var list = missiles;
            if (list == null)
            {
                return;
            }

            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var missile = Raw.At(list, i);
                if (missile.resting && !missile.ending)
                {
                    var entry = missile.entry!;
                    if (entry.X >= minX && entry.X <= maxX && entry.Y >= minY && entry.Y <= maxY)
                    {
                        missile.groundGone = true;
                    }
                }
            }
        }

        /// <summary>Takes the ended missiles out of the list, keeping the order of the rest.</summary>
        private static void Pack(List<Missile> list)
        {
            dropped = false;
            var kept = 0;
            var count = list.Count;
            for (var i = 0; i < count; i++)
            {
                var missile = Raw.At(list, i);
                if (!missile.ending)
                {
                    Raw.Put(list, kept, missile);
                    kept++;
                }
            }

            Raw.Truncate(list, kept);
        }
    }
}
