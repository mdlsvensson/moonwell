namespace Moonwell.Engine
{
    /// <summary>What the package tracks: the entries it gives to things with a place, and the count of them that decides whether the cycle runs. For now it tracks gizmos.</summary>
    internal static class Tracking
    {
        /// <summary>The cycle; made on first use after an Install.</summary>
        private static Cycle? cycle;

        /// <summary>The timer that runs the cycle every step; null while nothing is tracked.</summary>
        private static Timer? timer;

        /// <summary>How many things are tracked.</summary>
        private static int tracked;

        /// <summary>The id the last entry was given.</summary>
        private static int lastId;

        /// <summary>The cycle, made on first use.</summary>
        public static Cycle Beat
        {
            get
            {
                if (cycle == null)
                {
                    cycle = new Cycle();
                }

                return cycle;
            }
        }

        /// <summary>The step the cycle is at.</summary>
        public static int StepNow
        {
            get { return Beat.StepNow; }
        }

        /// <summary>Whether the cycle's timer is running.</summary>
        public static bool Running
        {
            get { return timer != null && !timer.HasEnded; }
        }

        /// <summary>Forgets the cycle and the count; Interactions.Install calls it, as an install starts the package afresh.</summary>
        public static void Reset()
        {
            cycle = null;
            timer = null;
            tracked = 0;
            lastId = 0;
        }

        /// <summary>Gives a thing its entry: the next id and its place, in the grid and with a place. The first thing tracked starts the cycle's timer.</summary>
        public static Entry Begin(Actor thing, int kind, float x, float y)
        {
            var grid = Interactions.Grid;
            lastId++;
            var entry = new Entry(lastId, thing, kind);
            entry.X = x;
            entry.Y = y;
            entry.HasPlace = true;
            grid.Add(entry);
            tracked++;
            if (tracked == 1 && !Running)
            {
                timer = Timer.Every(Interactions.Step, Run, Runtime.Root);
            }

            return entry;
        }

        /// <summary>Tells the package a thing is ending, before its own reaction runs: it has no place from now. It is still in the grid.</summary>
        public static void Ending(Entry entry)
        {
            entry.HasPlace = false;
        }

        /// <summary>Takes an ending thing out of the grid and stops tracking it. The last one tracked ends the cycle's timer.</summary>
        public static void Release(Entry entry)
        {
            Interactions.Grid.Remove(entry);
            if (tracked > 0)
            {
                tracked--;
            }

            if (tracked == 0 && timer != null)
            {
                timer.End();
                timer = null;
            }
        }

        /// <summary>The cycle's timer action: one step.</summary>
        private static void Run()
        {
            Beat.Run();
        }
    }
}
